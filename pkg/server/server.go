package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
)

var queue = make(map[string][]byte)

func Serve(ctx context.Context, address string) {
	g := gin.New()

	root := g.Group("/")
	root.PUT("/", putCollectorOutput)
	root.GET("/", getQueuedCollectors)
	root.GET("/collector/:id", getCollectorOutput)

	srvr := http.Server{Addr: address, Handler: g}
	go func() {
		srvr.ListenAndServe()
	}()
}

func putCollectorOutput(c *gin.Context) {
	collectorID := c.Request.Header.Get("collector-id")

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(500)
		return
	}

	queue[collectorID] = body

	fmt.Printf("queue = %#v\n", queue)
	c.Status(201)
}

func getCollectorOutput(c *gin.Context) {
	encoded := base64.StdEncoding.EncodeToString(queue[c.Param("id")])
	c.String(200, encoded)
}

func getQueuedCollectors(c *gin.Context) {
	keys := make([]string, 0, len(queue))
	for k := range queue {
		keys = append(keys, k)
	}

	c.JSON(200, keys)
}
