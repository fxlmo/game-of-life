package main

import (
	"strconv"
	"strings"
)

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell, keyChan <-chan rune) {

	select {
	case key := <-keyChan:
		println("Found this:")
		println(key)
	default:
	}

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

	for turns := 0; turns < p.turns; turns++ {
		//call world building function
		var inputChans []chan byte
		var outChans []chan byte
		for c := 0; c < p.threads; c++ {
			inputChans = append(inputChans, make(chan byte, p.imageWidth*(p.imageHeight/p.threads+2)))
			outChans = append(outChans, make(chan byte, p.imageWidth*(p.imageHeight/p.threads+2)))
		}
		//sends each row byte by byte (including halo lines)
		for w := 1; w <= p.threads; w++ {
			starty := (w - 1) * p.imageHeight / p.threads
			endy := w * p.imageHeight / p.threads
			//calculate first halo line
			for x := 0; x < p.imageWidth; x++ {

				haloY := starty - 1
				if haloY < 0 {
					haloY = p.imageHeight - 1
				}

				inputChans[w-1] <- world[haloY][x]
			}
			for y := starty; y < endy; y++ {
				for x := 0; x < p.imageWidth; x++ {
					inputChans[w-1] <- world[y][x]
				}
			}
			for x := 0; x < p.imageWidth; x++ {
				haloY := endy
				if haloY >= p.imageHeight {
					haloY = 0
				}
				inputChans[w-1] <- world[haloY][x]
			}
			go calcPgm(p, d, alive, inputChans[w-1], w, outChans[w-1])
		}

		//function to reconstruct world (use channels)
		for w := 0; w < p.threads; w++ {
			for y := w * p.imageHeight / p.threads; y < (w+1)*p.imageHeight/p.threads; y++ {
				for x := 0; x < p.imageWidth; x++ {
					world[y][x] = <-outChans[w]
				}
			}
		}
		//fmt.Printf("turn %d\n", turns)
		//visualiseMatrix(world, p.imageWidth, p.imageHeight)
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
	d.io.command <-ioCheckIdle
	<-d.io.idle


	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}

func calcPgm(p golParams, d distributorChans, alive chan []cell, inputChan chan byte, id int, outChan chan byte) {
	// initiate temporary world structure and world bit
	tempWorld := make([][]int, p.imageHeight/p.threads+2)
	world := make([][]byte, p.imageHeight/p.threads+2)
	for i := range tempWorld {
		tempWorld[i] = make([]int, p.imageWidth)
		world[i] = make([]byte, p.imageWidth)
	}

	//populate the world with values from the input channel
	for y := 0; y < p.imageHeight/p.threads+2; y++ {
		for x := 0; x < p.imageWidth; x++ {
			world[y][x] = <-inputChan
		}
	}

	// Calculate the new state of Game of Life (1 turn).
	for y := 1; y < p.imageHeight/p.threads+1; y++ {
		for x := 0; x < p.imageWidth; x++ {
			// collect neighbours
			liveNeighbours := 0
			for i := -1; i <= 1; i++ {
				for j := -1; j <= 1; j++ {
					if j != 0 || i != 0 {
						a, b := y+i, x+j
						b = (b + p.imageWidth) % (p.imageWidth)
						if world[a][b] == 0xFF {
							liveNeighbours += 1
						}
					}
				}
			}
			tempWorld[y][x] = liveNeighbours
		}
	}
	for a := 1; a < p.imageHeight/p.threads+1; a++ {
		for b := 0; b < p.imageWidth; b++ {
			if tempWorld[a][b] == 3 {
				world[a][b] = 0xFF
			} else if tempWorld[a][b] != 2 {
				world[a][b] = 0
			}
			tempWorld[a][b] = 0
		}
	}
	for y := 1; y < (p.imageHeight/p.threads + 1); y++ {
		for x := 0; x < p.imageWidth; x++ {
			outChan <- world[y][x]
		}
	}
}
