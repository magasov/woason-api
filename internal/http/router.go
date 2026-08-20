package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"woason-api/internal/admin"
	"woason-api/internal/auth"
	"woason-api/internal/catalog"
	"woason-api/internal/chat"
	"woason-api/internal/config"
	"woason-api/internal/delivery"
	"woason-api/internal/httpx"
	"woason-api/internal/models"
	"woason-api/internal/order"
	"woason-api/internal/payment"
	"woason-api/internal/shop"
	"woason-api/internal/store"
	"woason-api/internal/upload"
	"woason-api/internal/ws"
)

type Deps struct {
	Config config.Config
	Store  *store.Store
	Tokens *auth.Tokens
	Hub    *ws.Hub
	Pay    *payment.Client
}

func NewRouter(d Deps) http.Handler {
	authH := &auth.Handler{Store: d.Store, Tokens: d.Tokens}
	catH := &catalog.Handler{Store: d.Store}
	shopH := &shop.Handler{Store: d.Store}
	ordH := &order.Handler{Store: d.Store, Pay: d.Pay, Hub: d.Hub, Config: d.Config}
	chatH := &chat.Handler{Store: d.Store, Hub: d.Hub}
	admH := &admin.Handler{Store: d.Store, Pay: d.Pay, Hub: d.Hub, Config: d.Config}
	upH := &upload.Handler{Dir: d.Config.UploadDir, PublicBaseURL: d.Config.PublicBaseURL}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(CORS(d.Config.FrontendURL))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Handle("/uploads/*", upload.FileServer(d.Config.UploadDir))

	r.Get("/ws", func(w http.ResponseWriter, req *http.Request) {
		token := req.URL.Query().Get("token")
		p, err := d.Tokens.ParseAccess(token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "нужна авторизация")
			return
		}
		u, err := d.Store.GetUserByID(req.Context(), p.ID)
		if err != nil || u == nil {
			httpx.Error(w, http.StatusUnauthorized, "сессия недействительна")
			return
		}
		if u.BannedAt != nil {
			httpx.Error(w, http.StatusForbidden, "аккаунт заблокирован")
			return
		}
		p.Role = u.Role
		d.Hub.Serve(w, req, p, func(user *models.Principal, peerID, text string) (*models.ChatMessage, string, string, error) {
			return chatH.SendMessage(req.Context(), user, peerID, text)
		})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/logout", authH.Logout)
		r.Post("/auth/refresh", authH.Refresh)
		r.Post("/payments/yookassa/webhook", ordH.YookassaWebhook)
		r.Get("/delivery/quote", delivery.QuoteHandler)

		r.Group(func(r chi.Router) {
			r.Use(OptionalAuth(d.Tokens))
			r.Get("/categories", catH.Categories)
			r.Get("/products", catH.Products)
			r.Get("/products/{id}", catH.Product)
			r.Get("/products/{id}/reviews", catH.ProductReviews)
			r.Get("/reels", catH.Reels)
			r.Get("/reels/{id}", catH.Reel)
			r.Get("/shops/{id}", shopH.Get)
			r.Get("/shops/{id}/products", shopH.Products)
			r.Get("/shops/{id}/reels", shopH.Reels)
			r.Get("/shops/{id}/stories", shopH.Stories)
		})

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(d.Tokens, d.Store.GetUserByID))
			r.Get("/me", authH.Me)
			r.Patch("/me", authH.PatchMe)
			r.Post("/uploads", upH.Upload)
			r.Post("/reels/{id}/like", catH.LikeReel)
			r.Post("/reels/{id}/comments", catH.CommentReel)

			r.Get("/cart", ordH.GetCart)
			r.Post("/cart", ordH.AddCart)
			r.Put("/cart", ordH.PutCart)
			r.Delete("/cart", ordH.ClearCart)
			r.Patch("/cart/{productId}", ordH.PatchCart)
			r.Delete("/cart/{productId}", ordH.DeleteCartItem)

			r.Get("/favorites", ordH.Favorites)
			r.Post("/favorites", ordH.AddFavorite)
			r.Delete("/favorites/{productId}", ordH.DeleteFavorite)

			r.Get("/cabinet/orders", ordH.CabinetOrders)
			r.Get("/cabinet/reviews", ordH.CabinetReviews)
			r.Get("/cabinet/reviews/pending", ordH.CabinetPendingReviews)
			r.Get("/cabinet/dashboard", ordH.CabinetDashboard)
			r.Post("/checkout", ordH.Checkout)
			r.Get("/orders/{id}", ordH.GetOrder)
			r.Post("/products/{id}/reviews", catH.CreateReview)

			r.Get("/chats", chatH.List)
			r.Get("/chats/{peerId}/messages", chatH.Messages)
			r.Post("/chats/{peerId}/messages", chatH.Send)
			r.Post("/chats/{peerId}/read", chatH.Read)

			r.Group(func(r chi.Router) {
				r.Use(RequireSeller())
				r.Get("/seller/dashboard", shopH.Dashboard)
				r.Patch("/seller/shop", shopH.PatchShop)
				r.Post("/seller/products", shopH.CreateProduct)
				r.Get("/seller/products", shopH.SellerProducts)
				r.Post("/seller/stories", shopH.CreateStory)
				r.Post("/seller/reels", shopH.CreateReel)
				r.Get("/seller/orders", ordH.SellerOrders)
				r.Get("/seller/reviews", shopH.SellerReviews)
				r.Post("/seller/reviews/{reviewId}/reply", shopH.ReplyReview)
				r.Post("/seller/orders/{id}/label", ordH.PrintLabel)
				r.Post("/seller/orders/{id}/status", ordH.SellerStatus)
			})

			r.Group(func(r chi.Router) {
				r.Use(RequireRole(models.RoleAdmin))
				r.Get("/admin/stats", admH.Stats)
				r.Get("/admin/users", admH.Users)
				r.Patch("/admin/users/{id}", admH.PatchUser)
				r.Get("/admin/shops", admH.Shops)
				r.Patch("/admin/shops/{id}", admH.PatchShop)
				r.Get("/admin/products", admH.Products)
				r.Patch("/admin/products/{id}", admH.PatchProduct)
				r.Delete("/admin/products/{id}", admH.DeleteProduct)
				r.Get("/admin/orders", admH.Orders)
				r.Patch("/admin/orders/{id}", admH.PatchOrder)
				r.Get("/admin/payments", admH.Payments)
				r.Post("/admin/payments/{id}/refund", admH.Refund)
				r.Get("/admin/chats", admH.Chats)
				r.Get("/admin/chats/{threadId}/messages", admH.ChatMessages)
				r.Post("/admin/chats/messages/{id}/hide", admH.HideMessage)
			})
		})
	})

	return r
}
