package stats

import (
	"strconv"
)

type RequestData struct {
	Page   int
	Limit  int
	Offset int
}

type Pagination struct {
	TotalItems             int   `json:"total_items"`
	CurrentPage            int   `json:"current_page"`
	Limit                  int   `json:"limit"`
	TotalPages             int   `json:"total_pages"`
	Offset                 int   `json:"offset"`
	PrevPage               int   `json:"prev_page"`
	NextPage               int   `json:"next_page"`
	HasPrev                bool  `json:"-"`
	HasNext                bool  `json:"-"`
	HasDivider             bool  `json:"-"`
	CurrentPageIsInDivider bool  `json:"-"`
	FirstFew               []int `json:"-"`
	LastFew                []int `json:"-"`
}

type PageLimitOffsetData struct {
	Page   int
	Limit  int
	Offset int
}

func GetPageLimitOffsetFromRequestData(page string) (*PageLimitOffsetData, error) {
	if page == "" {
		page = "1"
	}
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		return nil, err
	}
	if pageInt < 1 {
		pageInt = 1
	}

	limitInt := 20

	offset := 0
	if pageInt > 1 {
		offset = (pageInt - 1) * limitInt
	}

	out := PageLimitOffsetData{
		Page:   pageInt,
		Limit:  limitInt,
		Offset: offset,
	}

	return &out, nil
}

func GeneratePagination(page, limit, offset, totalItems int) *Pagination {
	p := Pagination{TotalItems: totalItems, CurrentPage: page, Limit: limit, Offset: offset}
	if totalItems < 1 {
		return &p
	}

	if offset >= totalItems {
		return &p
	}

	p.TotalPages = totalItems / limit
	if totalItems%limit > 0 {
		p.TotalPages++
	}

	if p.TotalPages > p.CurrentPage {
		p.HasNext = true
		p.NextPage = p.CurrentPage + 1
	}

	if p.CurrentPage > 1 {
		p.HasPrev = true
		p.PrevPage = p.CurrentPage - 1
	}

	if p.TotalPages > 5 {
		p.HasDivider = true
		for i := 1; i < 4; i++ {
			p.FirstFew = append(p.FirstFew, i)
		}
		count := 0
		for i := p.TotalPages; i > 5; i-- {
			if count > 3 {
				break
			}
			p.LastFew = append(p.LastFew, i)
			count++
		}

		if p.CurrentPage > 5 && p.CurrentPage <= (p.TotalPages-3) {
			p.CurrentPageIsInDivider = true
		}
	}

	return &p
}
