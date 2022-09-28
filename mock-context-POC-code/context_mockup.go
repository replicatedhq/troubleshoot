//Mockup code is meant to show proof of concept in leveraging context.WithTimeout; cancel processing when timeout expires.

package main

import (
	"context"
	"fmt"
	"time"
)

const timeout = 3 //timeout in seconds

func processOne(ctx context.Context) { // representative of Logs Function; this code demonstrates a process that exceeds timeout and gets cancelled.
	for {
		select {
		case <-ctx.Done():
			fmt.Println("timed out")
			err := ctx.Err()
			fmt.Println(err)
			return
		default:
			//Where log() code would reside.
			fmt.Println("ProcessOne -- Processing")
		}
		time.Sleep(500 * time.Millisecond) //just to help visualize testing

	}
}

func processTwo() { // processTwo is present as a example to show that procesing continues within the program after an upstream processing function is cancelled due to a context timout.
	fmt.Println("ProcessTwo -- doing stuff.")

	fmt.Println("ProcessTwo -- Completed Successfully")
	fmt.Println("Done,Done,Done!")
}

func main() {
	//context with timeout base configuration
	// --- Start ---
	ctxBackground, cancel := context.WithTimeout(context.Background(), timeout*time.Second)
	defer cancel()
	// -- Finish

	processOne(ctxBackground) //apply context to instance

	processTwo()
}

