package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/network", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"miners":   []string{"http://0chain.yaitoo.cn:7071", "http://0chain.yaitoo.cn:7072", "http://0chain.yaitoo.cn:7073", "http://0chain.yaitoo.cn:7074"},
			"sharders": []string{"http://0chain.yaitoo.cn:7171"},
		})
	})
	r.Run(":9091") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
