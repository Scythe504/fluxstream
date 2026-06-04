package commands

import (
	"fmt"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[38;2;244;63;94m"     // Rose-500
	colorGreen  = "\033[38;2;16;185;129m"    // Emerald-500
	colorYellow = "\033[38;2;245;158;11m"    // Amber-500
	colorBlue   = "\033[38;2;99;102;241m"    // Indigo-500
	colorCyan   = "\033[38;2;6;182;212m"     // Cyan-500
	colorPink   = "\033[38;2;244;114;182m"   // Pink-400
	colorPurple = "\033[38;2;192;132;252m"   // Purple-400
	colorGray   = "\033[38;2;115;115;115m"   // Gray-500
)

func colorize(color, text string) string {
	return color + text + colorReset
}

func printError(msg string) {
	fmt.Println(colorize(colorRed, "[ERROR] ") + msg)
}

func printSuccess(msg string) {
	fmt.Println(colorize(colorGreen, "[SUCCESS] ") + msg)
}

func printInfo(msg string) {
	fmt.Println(colorize(colorCyan, "[INFO] ") + msg)
}

func printStep(msg string) {
	fmt.Println(colorize(colorPink, "==> ") + msg)
}

func printSubstep(msg string) {
	fmt.Println(colorize(colorGray, "  --> ") + msg)
}

func printHeader(msg string) {
	fmt.Println("\n" + colorize(colorPurple, "=== "+msg+" ==="))
}

const asciiLogo = `███████╗██╗     ██╗   ██╗██╗  ██╗███████╗████████╗██████╗ ███████╗ █████╗ ███╗   ███╗
██╔════╝██║     ██║   ██║╚██╗██╔╝██╔════╝╚══██╔══╝██╔══██╗██╔════╝██╔══██╗████╗ ████║
███████╗██║     ██║   ██║ ╚███╔╝ ███████╗   ██║   ██████╔╝█████╗  ███████║██╔████╔██║
██╔═══╝╝██║     ██║   ██║ ██╔██╗ ╚════██║   ██║   ██╔══██╗██╔══╝  ██╔══██║██║╚██╔╝██║
██║     ███████╗╚██████╔╝██╔╝ ██╗███████║   ██║   ██║  ██║███████╗██║  ██║██║ ╚═╝ ██║
╚═╝     ╚══════╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝`

type RGB struct {
	R, G, B float64
}

func interpolate(c1, c2 RGB, t float64) RGB {
	return RGB{
		R: c1.R + (c2.R-c1.R)*t,
		G: c1.G + (c2.G-c1.G)*t,
		B: c1.B + (c2.B-c1.B)*t,
	}
}

func GetLogoString() string {
	pink := RGB{R: 244, G: 114, B: 182}   // Pink-400
	purple := RGB{R: 192, G: 132, B: 252} // Purple-400

	lines := strings.Split(asciiLogo, "\n")
	var result strings.Builder

	for _, line := range lines {
		runes := []rune(line)
		n := len(runes)
		for i, r := range runes {
			if n <= 1 {
				result.WriteString(string(r))
				continue
			}
			t := float64(i) / float64(n-1)
			color := interpolate(pink, purple, t)
			escape := fmt.Sprintf("\033[38;2;%d;%d;%dm%c\033[0m", int(color.R), int(color.G), int(color.B), r)
			result.WriteString(escape)
		}
		result.WriteString("\n")
	}

	return result.String()
}

func PrintLogo() {
	fmt.Print(GetLogoString())
}
