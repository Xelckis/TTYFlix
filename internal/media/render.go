package media

import (
	"bytes"
	"strconv"
)

type CellColor struct {
	rTop, gTop, bTop          byte
	rBottom, gBottom, bBottom byte
}

type FrameState struct {
	Width      int
	Height     int
	PrevFrame  [][]CellColor
	CurrFrame  [][]CellColor
	FirstFrame bool
	blockCache []byte
}

func NewFrameState(width, height int) *FrameState {
	prev := make([][]CellColor, height)
	curr := make([][]CellColor, height)
	for y := range height {
		prev[y] = make([]CellColor, width)
		curr[y] = make([]CellColor, width)
	}

	maxBlocks := width + 16
	blockCache := make([]byte, maxBlocks*3)
	for i := range maxBlocks {
		blockCache[i*3] = 0xe2
		blockCache[i*3+1] = 0x96
		blockCache[i*3+2] = 0x80
	}

	return &FrameState{
		Width:      width,
		Height:     height,
		PrevFrame:  prev,
		CurrFrame:  curr,
		FirstFrame: true,
		blockCache: blockCache,
	}
}

func writeCursorPos(buf *bytes.Buffer, y, x int) {
	buf.WriteString("\033[")
	buf.WriteString(strconv.Itoa(y))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(x))
	buf.WriteByte('H')
}

func writeColors(buf *bytes.Buffer, c CellColor) {
	buf.WriteString("\033[38;2;")
	buf.WriteString(strconv.Itoa(int(c.rTop)))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(int(c.gTop)))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(int(c.bTop)))
	buf.WriteString("m\033[48;2;")
	buf.WriteString(strconv.Itoa(int(c.rBottom)))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(int(c.gBottom)))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(int(c.bBottom)))
	buf.WriteByte('m')
}

func (f *FrameState) RenderDeltas(frameData []byte, out *bytes.Buffer) {
	width := f.Width
	height := f.Height

	// Structure image bytes into half-block cell matrix
	for y := range height {
		for x := range width {
			topIdx := ((y*2)*width + x) * 3
			botIdx := (((y*2)+1)*width + x) * 3

			var rTop, gTop, bTop byte
			var rBot, gBot, bBot byte

			if topIdx+2 < len(frameData) {
				rTop = frameData[topIdx]
				gTop = frameData[topIdx+1]
				bTop = frameData[topIdx+2]
			}
			if botIdx+2 < len(frameData) {
				rBot = frameData[botIdx]
				gBot = frameData[botIdx+1]
				bBot = frameData[botIdx+2]
			}

			f.CurrFrame[y][x] = CellColor{
				rTop:    rTop,
				gTop:    gTop,
				bTop:    bTop,
				rBottom: rBot,
				gBottom: gBot,
				bBottom: bBot,
			}
		}
	}

	for y := range height {
		rleCount := 0
		rleStartX := 0
		var rleColor CellColor

		flushRLE := func() {
			if rleCount == 0 {
				return
			}
			writeCursorPos(out, y+1, rleStartX+1)
			writeColors(out, rleColor)
			if rleCount*3 <= len(f.blockCache) {
				out.Write(f.blockCache[:rleCount*3])
			} else {
				for i := 0; i < rleCount; i++ {
					out.WriteString("▀")
				}
			}
			rleCount = 0
		}

		for x := range width {
			currColor := f.CurrFrame[y][x]
			prevColor := f.PrevFrame[y][x]

			if f.FirstFrame || currColor != prevColor {
				if rleCount == 0 {
					rleStartX = x
					rleColor = currColor
					rleCount = 1
				} else if currColor == rleColor {
					rleCount++
				} else {
					flushRLE()
					rleStartX = x
					rleColor = currColor
					rleCount = 1
				}
			} else {
				if rleCount > 0 {
					flushRLE()
				}
			}
		}

		if rleCount > 0 {
			flushRLE()
		}
	}

	for y := range height {
		copy(f.PrevFrame[y], f.CurrFrame[y])
	}
	f.FirstFrame = false
}
