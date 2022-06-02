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
			chromedp.Flag("headless", false), // mude para false para executar com navegador visível.
			chromedp.NoSandbox,
			chromedp.DisableGPU,
		)...,
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(
		alloc,
		chromedp.WithLogf(log.Printf), // remover comentário para depurar
	)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, c.generalTimeout)
	defer cancel()

	// Navega até o site do MPRR e realiza as seleções para baixar contracheques.
	c.makeNavigationAndSelections(ctx, c.year, c.month, "contracheque")
	paycheckFileName := c.downloadFilePath("contracheque")

	// Realiza o download das planilhas
	if err := c.exportWorksheet(ctx, paycheckFileName); err != nil {
		log.Fatalf("Erro ao tentar fazer download: %q", err)
	}
	log.Println("Download realizado com sucesso!")

	// Navega até o site do MPRR e realiza as seleções para baixar verbas indenizatórias.
	c.makeNavigationAndSelections(ctx, c.year, c.month, "indenizatorias")
	indenizationsFileName := c.downloadFilePath("indenizatorias")

	// Realiza o download das planilhas
	if err := c.exportWorksheet(ctx, indenizationsFileName); err != nil {
		log.Fatalf("Erro ao tentar fazer download: %q", err)
	}
	log.Println("Download realizado com sucesso!")

	return []string{paycheckFileName, indenizationsFileName}, nil
}

func (c crawler) makeNavigationAndSelections(ctx context.Context, year, month, report string) {
	log.Println("\nNavegando até o site do MPRR...")
	if err := c.siteNavigation(ctx); err != nil {
		log.Fatalf("Erro ao tentar navegar até o site do MPRR: %q", err)
	}
	log.Println("Navegação realizada com sucesso!")

	log.Printf("Selecionando o mês %s...", month)
	if err := c.selectMonth(ctx, month); err != nil {
		log.Fatalf("Erro ao tentar selecionar o mês: %q", err)
	}
	log.Println("Mês selecionado com sucesso!")

	log.Printf("Selecionando o ano %s...", year)
	if err := c.selectYear(ctx, year); err != nil {
		log.Fatalf("Erro ao tentar selecionar o ano: %q", err)
	}
	log.Println("Ano selecionado com sucesso!")

	reportMap := map[string]string{
		"contracheque":   "1",
		"indenizatorias": "6",
	}

	log.Printf("Selecionando %s...", report)
	if err := c.selectReport(ctx, reportMap[report]); err != nil {
		log.Fatalf("Erro ao tentar selecionar %s: %q", report, err)
	}
	log.Println("Seleção realizada com sucesso!")
}

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

func (c crawler) selectMonth(ctx context.Context, month string) error {
	const monthSelector = `#mes`

	return chromedp.Run(
		ctx,

		chromedp.SetValue(monthSelector, month, chromedp.ByQuery),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}

func (c crawler) selectYear(ctx context.Context, year string) error {
	const yearSelector = `#ano`

	return chromedp.Run(
		ctx,

		chromedp.SetValue(yearSelector, year, chromedp.ByQuery),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}

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

func (c crawler) exportWorksheet(ctx context.Context, fileName string) error {
	tctx, tcancel := context.WithTimeout(ctx, 30*time.Second)
	defer tcancel()

	log.Println("Clicando no botão de emitir planilha...")
	if err := c.clickInEmitButton(tctx); err != nil {
		return fmt.Errorf("Erro ao tentar clicar no botão de emitir planilha: %v", err)
	}
	log.Println("Botão de emitir planilha clicado com sucesso!")

	log.Println("Verificando se há planilha...")
	if !hasWorksheet(tctx) {
		log.Println("Não há planilha para a data selecionada!")
		os.Exit(4)
	}

	log.Println("Clicando no botão de download...")
	if err := c.clickInDownloadButton(tctx); err != nil {
		return fmt.Errorf("Erro ao tentar clicar no botão de download: %v", err)
	}
	log.Println("Botão de download clicado com sucesso!")

	time.Sleep(c.donwloadTimeout)

	if err := renameDownload(c.outputFolder, fileName); err != nil {
		return fmt.Errorf("Erro ao tentar renomear o arquivo para (%s): %v", fileName, err)
	}
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return fmt.Errorf("download do arquivo de %s não realizado", fileName)
	}
	return nil
}

func (c crawler) clickInEmitButton(ctx context.Context) error {
	const buttonSelector = `/html/body/div[1]/div/div/section[2]/div/div/div[6]/div/div/div/div/div/div/div[2]/form/button`
	return chromedp.Run(
		ctx,
		chromedp.Click(buttonSelector, chromedp.BySearch),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}

func (c crawler) clickInDownloadButton(ctx context.Context) error {
	const buttonSelector = `/html/body/a[1]`
	return chromedp.Run(
		ctx,
		chromedp.Click(buttonSelector, chromedp.BySearch),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}

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

func renameDownload(output, fileName string) error {
	// Identifica qual foi o ultimo arquivo
	files, err := os.ReadDir(output)
	if err != nil {
		return fmt.Errorf("erro lendo diretório %s: %v", output, err)
	}
	var newestFPath string
	for _, f := range files {
		fPath := filepath.Join(output, f.Name())
		fi, err := os.Stat(fPath)
		if err != nil {
			return fmt.Errorf("erro obtendo informações sobre arquivo %s: %v", fPath, err)
		}

		fileTime, _ := time.Parse(time.UnixDate, fi.ModTime().Format(time.UnixDate))
		newTime, _ := time.Parse(time.UnixDate, time.Now().Format(time.UnixDate))
		sub := newTime.Sub(fileTime).Seconds()

		//Verifica se algum arquivo foi baixado nos últimos 40 segundos
		if sub <= 40 {
			newestFPath = fPath
		}
	}
	// Se estiver vazio, é porque nenhum arquivo foi baixado nos últimos 40 segundos
	if newestFPath == "" {
		return fmt.Errorf("nenhum arquivo foi baixado")
	}
	// Renomeia o ultimo arquivo modificado.
	if err := os.Rename(newestFPath, fileName); err != nil {
		return fmt.Errorf("erro renomeando último arquivo modificado (%s)->(%s): %v", newestFPath, fileName, err)
	}
	return nil
}
