package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"github.com/dadosjusbr/coletor-mprr/status"
)

type crawler struct {
	donwloadTimeout  time.Duration
	generalTimeout   time.Duration
	timeBetweenSteps time.Duration
	year             string
	month            string
	outputFolder     string
}

func (c crawler) crawl() ([]string, error) {
	// Chromedp setup.
	log.SetOutput(os.Stderr) // Enviando logs para o stderr para não afetar a execução do coletor.
	alloc, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36"),
			chromedp.Flag("headless", true), // mude para false para executar com navegador visível.
			chromedp.NoSandbox,
			chromedp.DisableGPU,
		)...,
	)
	defer allocCancel()

	//Criando o contexto do chromedp
	ctx, cancel := chromedp.NewContext(
		alloc,
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	//Anexando ao contexto o timeout
	ctx, cancel = context.WithTimeout(ctx, c.generalTimeout)
	defer cancel()

	// Navega até o site do MPRR e realiza as seleções para baixar contracheques.
	if err := c.makeNavigationAndSelections(ctx, c.year, c.month, "contracheque"); err != nil {
		return nil, err
	}
	paycheckFileName := c.downloadFilePath("contracheque")

	// Realiza o download das planilhas
	if err := c.exportWorksheet(ctx, paycheckFileName); err != nil {
		return nil, err
	}
	log.Println("Download realizado com sucesso!")

	// Navega até o site do MPRR e realiza as seleções para baixar verbas indenizatórias.
	if err := c.makeNavigationAndSelections(ctx, c.year, c.month, "indenizatorias"); err != nil {
		return nil, err
	}
	indenizationsFileName := c.downloadFilePath("indenizatorias")

	// Realiza o download das planilhas
	if err := c.exportWorksheet(ctx, indenizationsFileName); err != nil {
		return nil, err
	}
	log.Println("Download realizado com sucesso!")

	return []string{paycheckFileName, indenizationsFileName}, nil
}

//Realiza a navegação até o site do MPRR e realiza as seleções de ano, mês e tipo de relatório.
func (c crawler) makeNavigationAndSelections(ctx context.Context, year, month, report string) error {
	//Realizando a navegação até o site do MPRR
	log.Println("\nNavegando até o site do MPRR...")
	if err := c.siteNavigation(ctx); err != nil {
		return fmt.Errorf("Erro ao tentar navegar até o site do MPRR: %v", err)
	}
	log.Println("Navegação realizada com sucesso!")

	//Realizando a seleção do mês
	log.Printf("Selecionando o mês %s...", month)
	if err := chromedp.Run(
		ctx,
		chromedp.SetValue("#mes", month, chromedp.ByQuery),
		chromedp.Sleep(c.timeBetweenSteps),
	); err != nil {
		return fmt.Errorf("Erro ao tentar selecionar o mês: %v", err)
	}
	log.Println("Mês selecionado com sucesso!")

	//Realizando a seleção do ano
	log.Printf("Selecionando o ano %s...", year)
	if err := chromedp.Run(
		ctx,
		chromedp.SetValue("#ano", year, chromedp.ByQuery),
		chromedp.Sleep(c.timeBetweenSteps),
	); err != nil {
		return fmt.Errorf("Erro ao tentar selecionar o ano: %v", err)
	}
	log.Println("Ano selecionado com sucesso!")

	/* É necessário converter o tipo de relatório para um identificador. Esse
	identificador corresponde ao item do dropdown que será selecionado, podendo
	ser um contracheque ou verbas indenizatórias. */
	reportMap := map[string]string{
		"contracheque":   "1",
		"indenizatorias": "6",
	}

	log.Printf("Selecionando %s...", report)
	if err := c.selectReport(ctx, reportMap[report]); err != nil {
		return fmt.Errorf("Erro ao tentar selecionar %s: %v", report, err)
	}
	log.Println("Seleção realizada com sucesso!")

	return nil
}

//Realiza apenas a navegação até o site do MPRR, já selecionando a aba de contracheques.
func (c crawler) siteNavigation(ctx context.Context) error {
	const baseURL = `https://www.mprr.mp.br/web/transparencia/opcoesvencimentos`

	return chromedp.Run(
		ctx,
		chromedp.Navigate(baseURL),
		chromedp.Sleep(c.timeBetweenSteps),

		chromedp.Click(`//*[@id="tab6"]/div/div/ul[2]/li[2]/a`, chromedp.BySearch),
		chromedp.Sleep(c.timeBetweenSteps),

		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(c.outputFolder).
			WithEventsEnabled(true),
	)
}

//Seleciona o tipo de relatório a ser coletado
func (c crawler) selectReport(ctx context.Context, tableOption string) error {
	const boardSelector = `#quadro`

	return chromedp.Run(
		ctx,
		chromedp.SetValue(boardSelector, tableOption, chromedp.ByQuery),
		chromedp.Sleep(c.timeBetweenSteps),

		chromedp.SetValue(boardSelector, tableOption, chromedp.ByQuery),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}

//Realiza o download do arquivo de planilha
func (c crawler) exportWorksheet(ctx context.Context, fileName string) error {
	tctx, tcancel := context.WithTimeout(ctx, 50*time.Second)
	defer tcancel()

	buttonSelector := `/html/body/div[1]/div/div/section[2]/div/div/div[6]/div/div/div/div/div/div/div[2]/form/button`

	log.Println("Clicando no botão de emitir planilha...")
	if err := c.clickButton(tctx, buttonSelector); err != nil {
		return status.NewError(status.ERROR, fmt.Errorf("Erro ao tentar clicar no botão de emitir planilha: %v", err))
	}
	log.Println("Botão de emitir planilha clicado com sucesso!")

	log.Println("Verificando se há planilha...")
	if !hasWorksheet(tctx) {
		return status.NewError(status.DataUnavailable, fmt.Errorf("Não há planilha para a data selecionada!"))
	}

	buttonSelector = `/html/body/a[1]`

	log.Println("Clicando no botão de download...")
	if err := c.clickButton(tctx, buttonSelector); err != nil {
		return status.NewError(status.ERROR, fmt.Errorf("Erro ao tentar clicar no botão de download: %v", err))
	}
	log.Println("Botão de download clicado com sucesso!")

	time.Sleep(c.donwloadTimeout)

	if err := renameDownload(c.outputFolder, fileName); err != nil {
		return status.NewError(status.ERROR, fmt.Errorf("Erro ao tentar renomear o arquivo para (%s): %v", fileName, err))
	}

	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return status.NewError(status.ERROR, fmt.Errorf("download do arquivo de %s não realizado", fileName))
	}

	return nil
}

func (c crawler) clickButton(ctx context.Context, buttonSelector string) error {
	return chromedp.Run(
		ctx,
		chromedp.Click(buttonSelector, chromedp.BySearch),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}

//Verificar se há planilha para a data selecionada
func hasWorksheet(ctx context.Context) bool {
	const divSelector = "body > div.wrapper > div > div > section.content > div.alert.alert-error > div"
	tctx, tcancel := context.WithTimeout(ctx, 10*time.Second)
	defer tcancel()
	err := chromedp.Run(
		tctx,
		chromedp.Query(divSelector),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		return true
	}

	return false
}

func (c crawler) downloadFilePath(prefix string) string {
	return filepath.Join(
		c.outputFolder,
		fmt.Sprintf("membros-ativos-%s-%s-%s.xlsx", prefix, c.month, c.year),
	)
}

//Renomeia o arquivo após o download
func renameDownload(output, fileName string) error {
	// Identifica qual foi o ultimo arquivo
	files, err := os.ReadDir(output)
	if err != nil {
		return fmt.Errorf("erro lendo diretório %s: %v", output, err)
	}
	var newestFPath string
	var newestTime int64 = 0
	for _, f := range files {
		fPath := filepath.Join(output, f.Name())
		fi, err := os.Stat(fPath)
		if err != nil {
			return fmt.Errorf("erro obtendo informações sobre arquivo %s: %v", fPath, err)
		}

		currTime := fi.ModTime().Unix()
		if currTime > newestTime {
			newestTime = currTime
			newestFPath = fPath
		}
	}

	// Renomeia o ultimo arquivo modificado.
	if err := os.Rename(newestFPath, fileName); err != nil {
		return fmt.Errorf("erro renomeando último arquivo modificado (%s)->(%s): %v", newestFPath, fileName, err)
	}
	return nil
}
