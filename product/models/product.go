package models

type ProductMedia struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	AltText   string `json:"altText"`
	SortOrder int    `json:"sortOrder"`
}

type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	IsActive    bool   `json:"isActive"`
}

type ProductDimensions struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Depth  float64 `json:"depth"`
}

type ProductReview struct {
	Rating        int    `json:"rating"`
	Comment       string `json:"comment"`
	Date          string `json:"date"`
	ReviewerName  string `json:"reviewerName"`
	ReviewerEmail string `json:"reviewerEmail"`
}

type ProductMeta struct {
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Barcode   string `json:"barcode"`
	QRCode    string `json:"qrCode"`
}

type Product struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	Price                float64           `json:"price"`
	DiscountPercentage   float64           `json:"discountPercentage"`
	Rating               float64           `json:"rating"`
	Stock                int               `json:"stock"`
	Tags                 []string          `json:"tags"`
	Brand                string            `json:"brand"`
	SKU                  string            `json:"sku"`
	Weight               float64           `json:"weight"`
	Dimensions           ProductDimensions `json:"dimensions"`
	WarrantyInformation  string            `json:"warrantyInformation"`
	ShippingInformation  string            `json:"shippingInformation"`
	AvailabilityStatus   string            `json:"availabilityStatus"`
	Reviews              []ProductReview   `json:"reviews"`
	ReturnPolicy         string            `json:"returnPolicy"`
	MinimumOrderQuantity int               `json:"minimumOrderQuantity"`
	Meta                 ProductMeta       `json:"meta"`
	Thumbnail            string            `json:"thumbnail"`
	Images               []string          `json:"images"`
	AccountID            int               `json:"accountID"`
	CategoryID           string            `json:"categoryId"`
	PublishStatus        string            `json:"publishStatus"`
	ModerationStatus     string            `json:"moderationStatus"`
	ModerationReason     string            `json:"moderationReason"`
	Media                []ProductMedia    `json:"media"`
}

type ProductDocument = Product
