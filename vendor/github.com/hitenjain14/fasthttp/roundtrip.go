//go:build !js

package fasthttp

import "time"

func (t *transport) RoundTrip(hc *HostClient, req *Request, resp *Response) (retry bool, err error) {
	customSkipBody := resp.SkipBody
	customStreamBody := resp.StreamBody

	var deadline time.Time
	if req.timeout > 0 {
		deadline = time.Now().Add(req.timeout)
	}

	cc, err := hc.acquireConn(req.timeout, req.ConnectionClose())
	if err != nil {
		return false, err
	}
	conn := cc.c

	resp.parseNetConn(conn)

	writeDeadline := deadline
	if hc.WriteTimeout > 0 {
		tmpWriteDeadline := time.Now().Add(hc.WriteTimeout)
		if writeDeadline.IsZero() || tmpWriteDeadline.Before(writeDeadline) {
			writeDeadline = tmpWriteDeadline
		}
	}

	if err = conn.SetWriteDeadline(writeDeadline); err != nil {
		hc.closeConn(cc)
		return true, err
	}

	resetConnection := false
	if hc.MaxConnDuration > 0 && time.Since(cc.createdTime) > hc.MaxConnDuration && !req.ConnectionClose() {
		req.SetConnectionClose()
		resetConnection = true
	}

	bw := hc.acquireWriter(conn)
	err = req.Write(bw)

	if resetConnection {
		req.Header.ResetConnectionClose()
	}

	if err == nil {
		err = bw.Flush()
	}
	hc.releaseWriter(bw)

	// Return ErrTimeout on any timeout.
	if x, ok := err.(interface{ Timeout() bool }); ok && x.Timeout() {
		err = ErrTimeout
	}

	isConnRST := isConnectionReset(err)
	if err != nil && !isConnRST {
		hc.closeConn(cc)
		return true, err
	}

	readDeadline := deadline
	if hc.ReadTimeout > 0 {
		tmpReadDeadline := time.Now().Add(hc.ReadTimeout)
		if readDeadline.IsZero() || tmpReadDeadline.Before(readDeadline) {
			readDeadline = tmpReadDeadline
		}
	}

	if err = conn.SetReadDeadline(readDeadline); err != nil {
		hc.closeConn(cc)
		return true, err
	}

	if customSkipBody || req.Header.IsHead() {
		resp.SkipBody = true
	}
	if hc.DisableHeaderNamesNormalizing {
		resp.Header.DisableNormalizing()
	}

	br := hc.acquireReader(conn)
	err = resp.ReadLimitBody(br, hc.MaxResponseBodySize)
	if err != nil {
		hc.releaseReader(br)
		hc.closeConn(cc)
		// Don't retry in case of ErrBodyTooLarge since we will just get the same again.
		needRetry := err != ErrBodyTooLarge
		return needRetry, err
	}

	closeConn := resetConnection || req.ConnectionClose() || resp.ConnectionClose() || isConnRST
	if customStreamBody && resp.bodyStream != nil {
		rbs := resp.bodyStream
		resp.bodyStream = newCloseReader(rbs, func() error {
			hc.releaseReader(br)
			if r, ok := rbs.(*requestStream); ok {
				releaseRequestStream(r)
			}
			if closeConn || resp.ConnectionClose() {
				hc.closeConn(cc)
			} else {
				hc.releaseConn(cc)
			}
			return nil
		})
		return false, nil
	} else {
		hc.releaseReader(br)
	}

	if closeConn {
		hc.closeConn(cc)
	} else {
		hc.releaseConn(cc)
	}
	return false, nil
}
