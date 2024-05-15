package utils

import (
	"fmt"

	"github.com/common-nighthawk/go-figure"
	"nuance.xaas-logging.event-log-collector/pkg"
)

func PrintLogo() {
	nuanceLogo := figure.NewColorFigure("Nuance", "", "green", true)
	nuanceLogo.Print()
	ccssLite := figure.NewColorFigure("Event Log Collector", "", "blue", true)
	ccssLite.Print()
	version := figure.NewColorFigure(fmt.Sprintf("v%s", pkg.GetVersion()), "", "red", true)
	version.Print()
	fmt.Println()
}
