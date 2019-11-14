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
	tempWorld := make([][]byte, p.imageHeight)
	for i := range world {
		tempWorld[i] = make([]byte, p.imageWidth)
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
				tempWorld[y][x] = val
			}
		}
	}

	fmt.Printf("total turns = %d\n", p.turns)
	// Calculate the new state of Game of Life after the given number of turns.
	for turns := 0; turns < p.turns; turns++ {
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				// collect neighbours
				y_1 := y-1
				if y_1 < 0 {
					y_1 = p.imageHeight-1
				}
				x_1 := x-1
				if x_1 < 0 {
					x_1 = p.imageWidth-1
				}
				y1 := y+1
				if y1 >= p.imageHeight {
					y1 = 0
				}
				x1 := x+1
				if x1 >= p.imageWidth {
					x1 = 0
				}
				var neighbours []byte
				neighbours = append(neighbours, world[y][x1]) 	//1. x+1, y
				neighbours = append(neighbours, world[y][x_1])	//2. x-1, y
				neighbours = append(neighbours, world[y1][x]) 	//3. x, y+1
				neighbours = append(neighbours, world[y1][x1]) 	//4. x+1. y+1
				neighbours = append(neighbours, world[y1][x_1]) //5. x-1, y+1
				neighbours = append(neighbours, world[y_1][x]) 	//6. x, y-1
				neighbours = append(neighbours, world[y_1][x1]) //7. x+1, y-1
				neighbours = append(neighbours, world[y_1][x_1])//8. y-1, x-1

				// number of live neighbours
				liveNeighbours := 0
				for _, n := range neighbours {
					if n != 0 {
						liveNeighbours++
					}
				}
				change := false
				if liveNeighbours > 0 {
					fmt.Printf("Live neightbours for %d, %d, (%d) after turn %d: %d\n",x,y, world[y][x], turns, liveNeighbours)
					if (liveNeighbours < 2 || liveNeighbours > 3) && world[y][x] != 0{
						fmt.Printf("test for dead cell at %d, %d, turn %d\n", x, y, turns)
						tempWorld[y][x] = 0
						change = true
					}
					if (liveNeighbours == 3) && (world[y][x] == 0) {
						fmt.Printf("test for alive cell at %d, %d, turn %d\n", x, y, turns)
						tempWorld[y][x] = 255
						change = true
					}
				}

				//if world[y][x] != 0 {
				//	// case where cell is alive
				//	if liveNeighbours < 2 {
				//		fmt.Printf("dead cell at %d, %d, turn %d\n", x, y, turns)
				//		tempWorld[y][x] = 0
				//	} else if liveNeighbours > 3 {
				//		fmt.Printf("dead cell at %d, %d, turn %d\n", x, y, turns)
				//		tempWorld[y][x] = 0
				//	} else {
				//		tempWorld[y][x] = 0xFF
				//	}
				//} else {
				//	// case where cell is dead
				//	if liveNeighbours == 3 {
				//		fmt.Printf("Resurrected cell at %d %d\n",x,y)
				//		tempWorld[y][x] = 0xFF
				//	} else {
				//		tempWorld[y][x] = 0
				//	}
				//}
				if change {
					fmt.Printf("before %d\n", world[y][x])
					world[y][x] = tempWorld[y][x]
					fmt.Printf("after %d\n", world[y][x])
				}
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

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
