package models

import "time"

const (
	RoleBuyer  = "buyer"
	RoleSeller = "seller"
	RoleAdmin  = "admin"

	TradeRetail    = "retail"
	TradeWholesale = "wholesale"
	TradeDropship  = "dropship"

	StatusPlaced           = "placed"
	StatusAwaitingPayment  = "awaiting_payment"
	StatusPaid             = "paid"
	StatusAwaitingShipment = "awaiting_shipment"
	StatusLabelPrinted     = "label_printed"
	StatusInTransit        = "in_transit"
	StatusDelivered        = "delivered"
	StatusCancelled        = "cancelled"
	StatusRefunded         = "refunded"

	PayPending           = "pending"
	PayWaitingForCapture = "waiting_for_capture"
	PaySucceeded         = "succeeded"
	PayCanceled          = "canceled"
)

type User struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Phone        string     `json:"phone"`
	Avatar       string     `json:"avatar"`
	Role         string     `json:"role"`
	BannedAt     *time.Time `json:"bannedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	PasswordHash string     `json:"-"`
	Seller       *Shop      `json:"seller,omitempty"`
}

type Shop struct {
	ID          string    `json:"id"`
	ShopName    string    `json:"shopName"`
	Description string    `json:"description"`
	Logo        string    `json:"logo"`
	Banner      string    `json:"banner,omitempty"`
	City        string    `json:"city"`
	Phone       string    `json:"phone,omitempty"`
	Kind        string    `json:"kind"`
	Delivery    []string  `json:"delivery"`
	Hidden      bool      `json:"hidden,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
}

type Review struct {
	ID            string   `json:"id"`
	ProductID     string   `json:"productId"`
	OrderID       string   `json:"orderId,omitempty"`
	UserID        string   `json:"userId,omitempty"`
	Author        string   `json:"author"`
	Rating        int      `json:"rating"`
	Text          string   `json:"text"`
	Date          string   `json:"date"`
	Photos        []string `json:"photos,omitempty"`
	ProductTitle  string   `json:"productTitle,omitempty"`
	ProductImage  string   `json:"productImage,omitempty"`
	SellerReply   string   `json:"sellerReply,omitempty"`
	SellerReplyAt string   `json:"sellerReplyAt,omitempty"`
}

type PendingReview struct {
	OrderID     string `json:"orderId"`
	ProductID   string `json:"productId"`
	Title       string `json:"title"`
	Price       int    `json:"price"`
	Image       string `json:"image"`
	DeliveredAt string `json:"deliveredAt"`
}

type Product struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Price        int       `json:"price"`
	OldPrice     *int      `json:"oldPrice,omitempty"`
	Rating       float64   `json:"rating"`
	ReviewsCount int       `json:"reviewsCount"`
	Reviews      []Review  `json:"reviews,omitempty"`
	SellerKind   string    `json:"sellerKind"`
	Condition    string    `json:"condition"`
	Category     string    `json:"category"`
	Image        string    `json:"image"`
	Images       []string  `json:"images"`
	SellerID     string    `json:"sellerId"`
	SellerName   string    `json:"sellerName"`
	City         string    `json:"city"`
	WeightKg     float64   `json:"weightKg"`
	InStock      int       `json:"inStock"`
	Delivery     []string  `json:"delivery"`
	Tags         []string  `json:"tags"`
	TradeType    string    `json:"tradeType,omitempty"`
	Hidden       bool      `json:"hidden,omitempty"`
	CreatedAt    time.Time `json:"createdAt,omitempty"`
}

type ReelComment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

type Reel struct {
	ID          string        `json:"id"`
	ProductID   string        `json:"productId"`
	SellerID    string        `json:"sellerId"`
	SellerName  string        `json:"sellerName"`
	Title       string        `json:"title"`
	Caption     string        `json:"caption"`
	Likes       int           `json:"likes"`
	Comments    []ReelComment `json:"comments"`
	DurationSec int           `json:"durationSec"`
	Gradient    []string      `json:"gradient"`
	Liked       bool          `json:"liked,omitempty"`
}

type Story struct {
	ID        string `json:"id"`
	SellerID  string `json:"sellerId"`
	Image     string `json:"image"`
	Caption   string `json:"caption"`
	CreatedAt string `json:"createdAt"`
}

type CartItem struct {
	ProductID string   `json:"productId"`
	Qty       int      `json:"qty"`
	Product   *Product `json:"product,omitempty"`
}

type OrderItem struct {
	ProductID string `json:"productId"`
	Title     string `json:"title"`
	Price     int    `json:"price"`
	Qty       int    `json:"qty"`
	Image     string `json:"image"`
}

type Order struct {
	ID            string      `json:"id"`
	CreatedAt     time.Time   `json:"createdAt"`
	BuyerID       string      `json:"buyerId"`
	SellerID      string      `json:"sellerId"`
	Items         []OrderItem `json:"items"`
	City          string      `json:"city"`
	Address       string      `json:"address"`
	Delivery      string      `json:"delivery"`
	DeliveryPrice int         `json:"deliveryPrice"`
	ETA           string      `json:"eta"`
	TrackNumber   string      `json:"trackNumber,omitempty"`
	Status        string      `json:"status"`
	Total         int         `json:"total"`
}

type Payment struct {
	ID              string `json:"id"`
	OrderID         string `json:"orderId"`
	Amount          int    `json:"amount"`
	Status          string `json:"status"`
	ConfirmationURL string `json:"confirmationUrl,omitempty"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	ThreadID  string    `json:"threadId,omitempty"`
	SellerID  string    `json:"sellerId"`
	BuyerID   string    `json:"buyerId"`
	FromID    string    `json:"fromId"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
	Read      bool      `json:"read"`
	Hidden    bool      `json:"hidden,omitempty"`
}

type ChatThread struct {
	ID       string    `json:"id"`
	SellerID string    `json:"sellerId"`
	BuyerID  string    `json:"buyerId"`
	PeerID   string    `json:"peerId"`
	PeerName string    `json:"peerName"`
	LastText string    `json:"lastText"`
	LastAt   time.Time `json:"lastAt"`
	Unread   int       `json:"unread"`
}

type Principal struct {
	ID    string
	Email string
	Role  string
}

func Rub(kopecks int64) int {
	return int(kopecks / 100)
}

func Kopecks(rub int) int64 {
	return int64(rub) * 100
}

func PublicTradeType(trade string, viewer *Principal, sellerID string) string {
	if trade == TradeDropship {
		if viewer != nil && (viewer.Role == RoleAdmin || viewer.ID == sellerID) {
			return TradeDropship
		}
		return TradeRetail
	}
	return trade
}

type WeekPoint struct {
	Date string `json:"date"`
	Sum  int    `json:"sum"`
}

type BuyerDashboard struct {
	FavoritesCount int         `json:"favoritesCount"`
	CartCount      int         `json:"cartCount"`
	OrdersCount    int         `json:"ordersCount"`
	Spent          int         `json:"spent"`
	InTransit      int         `json:"inTransit"`
	WaitingReviews int         `json:"waitingReviews"`
	Week           []WeekPoint `json:"week"`
}

func Strs(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
