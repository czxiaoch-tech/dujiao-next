package container

import (
	"errors"
	"os"
	"strings"

	"github.com/dujiao-next/internal/constants"
	categoryapp "github.com/dujiao-next/internal/modules/catalog/category/application"
	productwrite "github.com/dujiao-next/internal/modules/catalog/product/application/write"
	"github.com/shopspring/decimal"
)

const mirrorSimulationEnv = "MIRROR_RECHARGE_SIMULATION"

func (c *Container) initMirrorSimulationIfEnabled() error {
	if strings.TrimSpace(os.Getenv(mirrorSimulationEnv)) != "1" {
		return nil
	}
	if c == nil || c.CategoryRepo == nil || c.CategoryService == nil || c.ProductRepo == nil ||
		c.ProductSKURepo == nil || c.ProductWriteService == nil || c.GiftCardRepo == nil {
		return errors.New("mirror simulation dependencies unavailable")
	}

	product, err := c.ProductRepo.GetBySlug("chatgpt-pro-20x", true)
	if err != nil {
		return err
	}
	if product == nil {
		var categoryID uint
		categories, err := c.CategoryRepo.List()
		if err != nil {
			return err
		}
		for i := range categories {
			if strings.EqualFold(strings.TrimSpace(categories[i].Slug), "chatgpt") {
				categoryID = categories[i].ID
				break
			}
		}
		if categoryID == 0 {
			category, err := c.CategoryService.Create(categoryapp.UpsertInput{
				Slug: "chatgpt",
				NameJSON: map[string]interface{}{"zh-CN": "ChatGPT", "zh-TW": "ChatGPT", "en-US": "ChatGPT"},
			})
			if err != nil {
				return err
			}
			categoryID = category.ID
		}

		active := true
		unlimited := constants.ManualStockUnlimited
		product, err = c.ProductWriteService.Create(productwrite.CreateProductInput{
			CategoryID: categoryID,
			Slug: "chatgpt-pro-20x",
			TitleJSON: map[string]interface{}{"zh-CN": "ChatGPT Pro 20X", "zh-TW": "ChatGPT Pro 20X", "en-US": "ChatGPT Pro 20X"},
			DescriptionJSON: map[string]interface{}{"zh-CN": "镜像充值模拟实验商品"},
			PriceAmount: decimal.RequireFromString("1098.80"),
			CostPriceAmount: decimal.Zero,
			PurchaseType: "member",
			StockDisplayMode: "exact",
			FulfillmentType: constants.FulfillmentTypeManual,
			ManualStockTotal: &unlimited,
			ManualFormSchemaJSON: map[string]interface{}{
				"fields": []interface{}{map[string]interface{}{
					"key": "session_json",
					"type": "textarea",
					"required": true,
					"max_len": 20000,
					"label": map[string]interface{}{
						"zh-CN": "ChatGPT Session JSON",
						"zh-TW": "ChatGPT Session JSON",
						"en-US": "ChatGPT Session JSON",
					},
				}},
			},
			SKUs: []productwrite.ProductSKUInput{{
				SKUCode: "PRO20X",
				SpecValuesJSON: map[string]interface{}{"zh-CN": "Pro 20X", "en-US": "Pro 20X"},
				PriceAmount: decimal.RequireFromString("1098.80"),
				CostPriceAmount: decimal.Zero,
				ManualStockTotal: unlimited,
				IsActive: &active,
			}},
			IsActive: &active,
		})
		if err != nil {
			return err
		}
	}

	sku, err := c.ProductSKURepo.GetByProductAndCode(product.ID, "PRO20X")
	if err != nil {
		return err
	}
	if sku == nil {
		return errors.New("mirror simulation PRO20X sku missing")
	}
	return nil
}
