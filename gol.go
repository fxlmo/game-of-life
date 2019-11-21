package main

import (
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
				world[y][x] = val
			}
		}
	}

	//call world building function
	var chans []chan [][]byte
	for c := 0; c < p.threads; c++ {
		chans = append(chans, make(chan [][]byte))
	}

	for w := 1; w <= p.threads; w++ {
		go calcPgm(p, d, alive, world, w, chans[w-1])
	}

	//function to reconstruct world
	for w := 0; w < p.threads; w++ {
		worldPart :=<-chans[w]
		for y := 0; y < p.imageHeight/p.threads; y++ {
			for x := 0; x < p.imageWidth; x++ {
				world[y + w*(p.imageHeight/p.threads)][x] = worldPart[y % (p.imageHeight/p.threads)][x]
			}
		}
	}

	// Create an empty slice to store coordinates of cells that are still alive after p.turns are done.
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}

func calcPgm(p golParams, d distributorChans, alive chan []cell, world [][]byte, id int, channel chan [][]byte) {
	// initiate temporary world structure
	tempWorld := make([][]int, p.imageHeight/p.threads)
	for i := range tempWorld {
		tempWorld[i] = make([]int, p.imageWidth)
	}

	// Calculate the new state of Game of Life after the given number of turns.
	for turns := 0; turns < p.turns; turns++ {
		for y := (id-1)*p.imageHeight/p.threads; y < id*p.imageHeight/p.threads; y++ {
			for x := 0; x < p.imageWidth; x++ {
				// collect neighbours
				liveNeighbours := 0
				for i := -1; i<= 1; i++ {
					for j := -1; j <= 1; j++ {
						if j != 0 || i != 0 {
							a, b := y+i, x+j
							a = (a + id*p.imageHeight/p.threads) % (id*p.imageHeight/p.threads)
							b = (b + p.imageWidth) % (p.imageWidth)
							if world[a][b] == 0xFF {
								liveNeighbours += 1
							}
						}
					}
				}
				tempWorld[y % (p.imageHeight/p.threads)][x] = liveNeighbours
			}
		}
		for a := 0; a < id*p.imageHeight/p.threads; a++ {
			for b := 0; b < p.imageWidth; b++ {
				if tempWorld[a % (p.imageHeight/p.threads)][b] == 3 {
					world[a][b] = 0xFF
				} else if tempWorld[a% (p.imageHeight/p.threads)][b] != 2 {
					world[a][b] = 0
				}
				tempWorld[a % (p.imageHeight/p.threads)][b] = 0
			}
		}
	}
	channel <-world
}
