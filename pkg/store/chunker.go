package store

import (
	"fmt"
	"io"
)

const chunkAlignment = 4096

type chunker struct {
	reader  io.Reader
	buffer  []byte
	chunk   []byte
	offset  int
	length  int
	minimum int
	average int
	maximum int
	eof     bool
}

func newChunker(reader io.Reader, minimum, average, maximum int) (*chunker, error) {
	if minimum <= 0 || average < minimum || maximum < average || average&(average-1) != 0 {
		return nil, fmt.Errorf("store: invalid chunk sizes %d/%d/%d", minimum, average, maximum)
	}
	return &chunker{
		reader:  reader,
		buffer:  make([]byte, 64*1024),
		chunk:   make([]byte, 0, maximum),
		minimum: minimum,
		average: average,
		maximum: maximum,
	}, nil
}

func (c *chunker) Next() ([]byte, error) {
	c.chunk = c.chunk[:0]
	var hash uint64
	for len(c.chunk) < c.maximum {
		value, err := c.nextByte()
		if err != nil {
			if err == io.EOF && len(c.chunk) > 0 {
				return c.chunk, nil
			}
			return nil, err
		}
		c.chunk = append(c.chunk, value)
		hash = (hash << 1) + chunkGear[value]
		if len(c.chunk) >= c.minimum && len(c.chunk)%chunkAlignment == 0 && hash&uint64(c.average-1) == 0 {
			break
		}
	}
	return c.chunk, nil
}

func (c *chunker) nextByte() (byte, error) {
	if c.offset < c.length {
		value := c.buffer[c.offset]
		c.offset++
		return value, nil
	}
	if c.eof {
		return 0, io.EOF
	}
	length, err := c.reader.Read(c.buffer)
	if err != nil && err != io.EOF {
		return 0, err
	}
	if length == 0 {
		c.eof = true
		return 0, io.EOF
	}
	c.offset = 1
	c.length = length
	if err == io.EOF {
		c.eof = true
	}
	return c.buffer[0], nil
}

var chunkGear = newChunkGear()

func newChunkGear() [256]uint64 {
	var table [256]uint64
	value := uint64(0x9e3779b97f4a7c15)
	for index := range table {
		value += 0x9e3779b97f4a7c15
		mixed := value
		mixed = (mixed ^ (mixed >> 30)) * 0xbf58476d1ce4e5b9
		mixed = (mixed ^ (mixed >> 27)) * 0x94d049bb133111eb
		table[index] = mixed ^ (mixed >> 31)
	}
	return table
}
