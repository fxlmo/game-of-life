package main

import (
	"fmt"
	"strconv"
	"strings"
)

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell) {

	// Create the 2D slice to store the world.
	world := make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")


	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	// initiate temporary world structure
	tempWorld := make([][]int, p.imageHeight)
	for i := range tempWorld {
		tempWorld[i] = make([]int, p.imageWidth)
	}

	// Calculate the new state of Game of Life after the given number of turns.
	for turns := 0; turns < p.turns; turns++ {
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				// collect neighbours
				liveNeighbours := 0
				for i := -1; i<= 1; i++ {
					for j := -1; j <= 1; j++ {
						if j != 0 || i != 0 {
							a, b := y+i, x+j
							a = (a + p.imageHeight) % (p.imageHeight)
							b = (b + p.imageWidth) % (p.imageWidth)
							if world[a][b] != 0 {
								liveNeighbours += 1
							}
						}
					}
				}
				tempWorld[y][x] = liveNeighbours
			}
		}
		for a := 0; a < p.imageHeight; a++ {
			for b := 0; b < p.imageWidth; b++ {
				if tempWorld[a][b] == 3 {
					world[a][b] = 0xFF
				} else if tempWorld[a][b] != 2 {
					world[a][b] = 0
				}
				tempWorld[a][b] = 0
			}
		}
	}


	// Create an empty slice to store coordinates of cells that are still alive after p.turns are done.
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				fmt.Printf("Alive cell at %d, %d\n", x, y)
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}

	// The distributor goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[x][y]
		}
	}

	// Request the io goroutine to write the image with the given filename.
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
