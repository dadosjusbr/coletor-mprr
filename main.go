package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dadosjusbr/coletor-mprr/status"
)

const (
	defaultFileDownloadTimeout = 20 * time.Second // Duração que o coletor deve esperar até que o download de cada um dos arquivos seja concluído
	defaultGeneralTimeout      = 6 * time.Minute  // Duração máxima total da coleta de todos os arquivos. Valor padrão calculado a partir de uma média de execuções ~4.5min
	defaulTimeBetweenSteps     = 5 * time.Second  //Tempo de espera entre passos do coletor."
)

func main() {
	if _, err := strconv.Atoi(os.Getenv("MONTH")); err != nil {
		status.ExitFromError(status.NewError(status.Unknown, fmt.Errorf("Invalid month (\"%s\"): %q", os.Getenv("MONTH"), err)))
	}
	month := os.Getenv("MONTH")

	if _, err := strconv.Atoi(os.Getenv("YEAR")); err != nil {
		status.ExitFromError(status.NewError(status.Unknown, fmt.Errorf("Invalid year (\"%s\"): %q", os.Getenv("YEAR"), err)))
	}
	year := os.Getenv("YEAR")

	outputFolder := os.Getenv("OUTPUT_FOLDER")
	if outputFolder == "" {
		outputFolder = "/output"
	}

	if err := os.Mkdir(outputFolder, os.ModePerm); err != nil && !os.IsExist(err) {
		status.ExitFromError(status.NewError(status.Unknown, fmt.Errorf("Error creating output folder(%s): %q", outputFolder, err)))
	}

	downloadTimeout := defaultFileDownloadTimeout
	if os.Getenv("DOWNLOAD_TIMEOUT") != "" {
		var err error
		downloadTimeout, err = time.ParseDuration(os.Getenv("DOWNLOAD_TIMEOUT"))
		if err != nil {
			status.ExitFromError(status.NewError(status.Unknown, fmt.Errorf("Invalid DOWNLOAD_TIMEOUT (\"%s\"): %q", os.Getenv("DOWNLOAD_TIMEOUT"), err)))
		}
	}

	generalTimeout := defaultGeneralTimeout
	if os.Getenv("GENERAL_TIMEOUT") != "" {
		var err error
		generalTimeout, err = time.ParseDuration(os.Getenv("GENERAL_TIMEOUT"))
		if err != nil {
			status.ExitFromError(status.NewError(status.Unknown, fmt.Errorf("Invalid GENERAL_TIMEOUT (\"%s\"): %q", os.Getenv("GENERAL_TIMEOUT"), err)))
		}
	}

	timeBetweenSteps := defaulTimeBetweenSteps
	if os.Getenv("TIME_BETWEEN_STEPS") != "" {
		var err error
		timeBetweenSteps, err = time.ParseDuration(os.Getenv("TIME_BETWEEN_STEPS"))
		if err != nil {
			status.ExitFromError(status.NewError(status.Unknown, fmt.Errorf("Invalid TIME_BETWEEN_STEPS (\"%s\"): %q", os.Getenv("TIME_BETWEEN_STEPS"), err)))
		}
	}

	c := crawler{
		donwloadTimeout:  downloadTimeout,
		generalTimeout:   generalTimeout,
		timeBetweenSteps: timeBetweenSteps,
		year:             year,
		month:            month,
		outputFolder:     outputFolder,
	}
	downloads, err := c.crawl()
	if err != nil {
		status.ExitFromError(err)
	}

	fmt.Println(strings.Join(downloads, "\n"))
}
