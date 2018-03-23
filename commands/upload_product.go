package commands

import (
	"fmt"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/om/api"
)

type UploadProduct struct {
	multipart       multipart
	logger          logger
	productsService productUploader
	Options         struct {
		File            string `long:"product"          short:"p"  required:"true" description:"path to product"`
		PollingInterval int    `long:"polling-interval" short:"pi"                 description:"interval (in seconds) at which to print status" default:"1"`
	}
	extractor extractor
}

//go:generate counterfeiter -o ./fakes/product_uploader.go --fake-name ProductUploader . productUploader
type productUploader interface {
	Upload(api.UploadProductInput) (api.UploadProductOutput, error)
	CheckProductAvailability(string, string) (bool, error)
}

//go:generate counterfeiter -o ./fakes/extractor.go --fake-name Extractor . extractor
type extractor interface {
	ExtractMetadata(string) (string, string, error)
}

func NewUploadProduct(multipart multipart, extractor extractor, productUploader productUploader, logger logger) UploadProduct {
	return UploadProduct{
		multipart:       multipart,
		logger:          logger,
		productsService: productUploader,
		extractor:       extractor,
	}
}

func (up UploadProduct) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command attempts to upload a product to the Ops Manager",
		ShortDescription: "uploads a given product to the Ops Manager targeted",
		Flags:            up.Options,
	}
}

func (up UploadProduct) Execute(args []string) error {
	if _, err := jhanda.Parse(&up.Options, args); err != nil {
		return fmt.Errorf("could not parse upload-product flags: %s", err)
	}

	productName, productVersion, err := up.extractor.ExtractMetadata(up.Options.File)
	if err != nil {
		return fmt.Errorf("failed to extract product metadata: %s", err)
	}

	prodAvailable, err := up.productsService.CheckProductAvailability(productName, productVersion)
	if err != nil {
		return fmt.Errorf("failed to check product availability: %s", err)
	}

	if prodAvailable {
		up.logger.Printf("product %s %s is already uploaded, nothing to be done.", productName, productVersion)
		return nil
	}

	up.logger.Printf("processing product")
	err = up.multipart.AddFile("product[file]", up.Options.File)
	if err != nil {
		return fmt.Errorf("failed to load product: %s", err)
	}

	submission, err := up.multipart.Finalize()
	if err != nil {
		return fmt.Errorf("failed to create multipart form: %s", err)
	}

	up.logger.Printf("beginning product upload to Ops Manager")

	_, err = up.productsService.Upload(api.UploadProductInput{
		ContentLength:   submission.Length,
		File:            submission.Content,
		ContentType:     submission.ContentType,
		PollingInterval: up.Options.PollingInterval,
	})
	if err != nil {
		return fmt.Errorf("failed to upload product: %s", err)
	}

	up.logger.Printf("finished upload")

	return nil
}
