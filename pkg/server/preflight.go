package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
)

var preflightQueue = make(map[string][]byte)

func ServePreflight(ctx context.Context, address string) {
	g := gin.New()

	root := g.Group("/")
	root.PUT("/", putPreflightOutput)
	root.GET("/", getQueuedPreflights)
	root.GET("/preflight/:id", getPreflightOutput)

	srvr := http.Server{Addr: address, Handler: g}
	go func() {
		srvr.ListenAndServe()
	}()
}

func putPreflightOutput(c *gin.Context) {
	preflightID := c.Request.Header.Get("collector-id")

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(500)
		return
	}

	preflightQueue[preflightID] = body

	fmt.Printf("preflightQueue = %#v\n", preflightQueue)
	c.Status(201)
}

func getPreflightOutput(c *gin.Context) {
	encoded := base64.StdEncoding.EncodeToString(preflightQueue[c.Param("id")])
	c.String(200, encoded)
}

func getQueuedPreflights(c *gin.Context) {
	keys := make([]string, 0, len(preflightQueue))
	for k := range preflightQueue {
		keys = append(keys, k)
	}

	c.JSON(200, keys)
}
