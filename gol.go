package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell, keyChan <-chan rune) {

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

	//create ticker
	done := make(chan int, 0)
	go tick(done, p, world)

	//call world building function
	var inputChans []chan byte
	var outChans []chan byte
	var topHalo []chan byte
	var botHalo []chan byte
	var doneChan chan int
	doneChan = make(chan int, p.threads+100)
	averageThread := p.imageHeight / p.threads
	for c := 0; c < p.threads-1; c++ {
		inputChans = append(inputChans, make(chan byte, p.imageWidth*(averageThread+2)))
		outChans = append(outChans, make(chan byte, p.imageWidth*(averageThread+2)))
		topHalo = append(topHalo, make(chan byte, p.imageWidth*2))
		botHalo = append(botHalo, make(chan byte, p.imageWidth*2))
	}
	if p.imageHeight%p.threads != 0 {
		inputChans = append(inputChans, make(chan byte, p.imageWidth*(p.imageHeight%p.threads+100)))
		outChans = append(outChans, make(chan byte, p.imageWidth*(p.imageHeight%p.threads+100)))
	} else {
		inputChans = append(inputChans, make(chan byte, p.imageWidth*(averageThread+2)))
		outChans = append(outChans, make(chan byte, p.imageWidth*(averageThread+2)))
	}
	topHalo = append(topHalo, make(chan byte, p.imageWidth*2))
	botHalo = append(botHalo, make(chan byte, p.imageWidth*2))

	//sends each row byte by byte (including halo lines)
	var flags = new(keyFlags)
	flags.masterTurn = 0
	flags.paused = false
	flags.quit = false
	flags.show = false

	for w := 1; w <= p.threads; w++ {
		starty := (w - 1) * averageThread
		endy := w * averageThread
		oddOne := false
		if w == p.threads {
			endy = p.imageHeight
			oddOne = true
		}

		//calculate first halo line
		for x := 0; x < p.imageWidth; x++ {
			haloY := starty - 1
			if haloY < 0 {
				haloY = p.imageHeight - 1
			}
			topHalo[w-1] <- world[haloY][x]
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
			botHalo[w-1] <- world[haloY][x]
		}

		//TODO: change false to a flag
		go worker(p, inputChans[w-1], w-1, outChans[w-1], oddOne, topHalo, botHalo, doneChan, flags)
	}

	numberDone := 0
	for numberDone < p.threads {
		select {
		case currentAlive := <-done:
			fmt.Printf("alive: %d\n", currentAlive)
		case <-doneChan:
			numberDone++
		case key := <-keyChan:
			if key == 'q' {
				//visualise board and then quit
				//visualiseMatrix(world, p.imageWidth, p.imageHeight)
				flags.quit = true
				println("Quitting...")
				StopControlServer()
				os.Exit(0)
			}
			//key is s (115)
			if key == 's' {
				//visualise board
				flags.show = true
				println("Board at turn " + string(p.turns))
				//visualiseMatrix(world, p.imageWidth, p.imageHeight)
			}
			//key is p (112)
			if key == 'p' {
				//pause and print current term
				//If paused again, resume.
				//visualiseMatrix(world, p.imageWidth, p.imageHeight)
				flags.paused = true
				println("Paused")
				pause := true
				for pause {
					select {
					case pauseKey := <-keyChan:
						if pauseKey == 'p' {
							pause = false
						}
					default:
					}
				}
				println("Continuing")
			}
		default:
		}
	}

	//function to reconstruct world (use channels)
	for w := 0; w < p.threads-1; w++ {
		for y := w * averageThread; y < (w+1)*averageThread; y++ {
			for x := 0; x < p.imageWidth; x++ {
				world[y][x] = <-outChans[w]
			}
		}
	}

	for y := averageThread * (p.threads - 1); y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			world[y][x] = <-outChans[p.threads-1]
		}
	}
	//fmt.Printf("turn %d\n", turns)
	//visualiseMatrix(world, p.imageWidth, p.imageHeight)

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

func worker(p golParams, inputChan chan byte, id int, outChan chan byte, oddOne bool, topHalo []chan byte, botHalo []chan byte, doneChan chan int, flags *keyFlags) {
	// initiate temporary world structure and world bit
	worldSize := p.imageHeight / p.threads
	if oddOne && p.imageHeight%p.threads != 0 {
		worldSize = p.imageHeight - worldSize*(p.threads-1)
	}
	tempWorld := make([][]int, worldSize+2)
	world := make([][]byte, worldSize+2)
	for i := range tempWorld {
		tempWorld[i] = make([]int, p.imageWidth)
		world[i] = make([]byte, p.imageWidth)
	}

	//populate the world with values from the input channel
	for y := 1; y < worldSize+1; y++ {
		for x := 0; x < p.imageWidth; x++ {
			world[y][x] = <-inputChan
		}
	}

	for turns := 0; turns < p.turns; turns++ {
		//adjust halo lines from other workers
		for x := 0; x < p.imageWidth; x++ {
			world[0][x] = <-topHalo[id]
			world[worldSize+1][x] = <-botHalo[id]
		}

		// Calculate the new state of Game of Life (1 turn).
		for y := 1; y < worldSize+1; y++ {
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

		for a := 1; a < worldSize+1; a++ {
			for b := 0; b < p.imageWidth; b++ {
				if tempWorld[a][b] == 3 {
					world[a][b] = 0xFF
				} else if tempWorld[a][b] != 2 {
					world[a][b] = 0
				}
				tempWorld[a][b] = 0
			}
		}

		//Output Halo lines to appropriate channels
		for x := 0; x < p.imageWidth; x++ {
			botId := ((id - 1) + p.threads) % (p.threads)
			botHalo[botId] <- world[1][x]
			topHalo[(id+1)%p.threads] <- world[worldSize][x]
		}

		//sync all workers with master worker
		if id == 0 {
			flags.masterTurn++
		} else {
			for turns > flags.masterTurn {
				time.Sleep(1 * time.Nanosecond)
			}
		}
	}

	//return calculated world
	for y := 1; y < (worldSize + 1); y++ {
		for x := 0; x < p.imageWidth; x++ {
			outChan <- world[y][x]
		}
	}
	doneChan <- 1
}

func tick(done chan int, p golParams, world [][]byte) {
	t := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-t.C:
			finalAlive := 0
			for y := 0; y < p.imageHeight; y++ {
				for x := 0; x < p.imageWidth; x++ {
					if world[y][x] != 0 {
						finalAlive++
					}
				}
			}
			done <- finalAlive
		}
	}
}
