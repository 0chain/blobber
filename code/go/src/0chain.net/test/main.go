package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"0chain.net/badgerdbstore"
)

func main() {
	fmt.Println("Hello World")
	badgerdbstore.SetupStorageProvider("./test")
	var i int64
	wg := &sync.WaitGroup{}
	mutex := &sync.Mutex{}
	store := badgerdbstore.GetStorageProvider()
	for i = 0; i < 3; i++ {
		wg.Add(1)
		go func(i int64) {
			fmt.Println("Trying to read" + strconv.FormatInt(i, 10))
			ctx := store.WithConnection(context.Background())
			mutex.Lock()
			res, err := store.ReadBytes(ctx, "test")
			val := 0
			if err == nil {
				fmt.Println("Existing value in  DB: " + string(res))
				val, _ = strconv.Atoi(string(res))
				val++
				fmt.Println("Value to be stored:" + strconv.FormatInt(int64(val), 10))
			}
			store.WriteBytes(ctx, "test", []byte(strconv.FormatInt(int64(val), 10)))
			err = store.Commit(ctx)
			if err != nil {
				fmt.Println("Error in commit." + err.Error())
			}
			ctx.Done()
			wg.Done()
			//time.Sleep(1 * time.Second)
			mutex.Unlock()
		}(i)
	}
	wg.Wait()
	ctx := store.WithReadOnlyConnection(context.Background())
	res, _ := store.ReadBytes(ctx, "test")
	fmt.Println(string(res))
	store.Discard(ctx)
	ctx.Done()
}
