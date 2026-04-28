package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/puzakov/gophermart-loyalty-exam/internal/auth"
	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
	mw "github.com/puzakov/gophermart-loyalty-exam/internal/httpapi/middleware"
	"github.com/puzakov/gophermart-loyalty-exam/internal/usecase"
	"github.com/puzakov/gophermart-loyalty-exam/internal/validate"
)

type Server struct {
	tokens  *auth.TokenManager
	authUC  *usecase.AuthUsecase
	orders  *usecase.OrdersUsecase
	balance *usecase.BalanceUsecase
}

func NewServer(tokens *auth.TokenManager, authUC *usecase.AuthUsecase, orders *usecase.OrdersUsecase, balance *usecase.BalanceUsecase) *Server {
	return &Server{
		tokens:  tokens,
		authUC:  authUC,
		orders:  orders,
		balance: balance,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(mw.Gzip)

	r.Post("/api/user/register", s.handleRegister)
	r.Post("/api/user/login", s.handleLogin)
	r.Post("/api/user/token/refresh", s.handleRefresh)

	r.Group(func(ar chi.Router) {
		ar.Use(mw.RequireAuth(s.tokens))
		ar.Post("/api/user/orders", s.handleOrdersSubmit)
		ar.Get("/api/user/orders", s.handleOrdersList)
		ar.Get("/api/user/balance", s.handleBalance)
		ar.Post("/api/user/balance/withdraw", s.handleWithdraw)
		ar.Get("/api/user/withdrawals", s.handleWithdrawals)
	})

	return r
}

type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	t, err := s.authUC.Register(r.Context(), c.Login, c.Password, time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  t.AccessToken,
		"refresh_token": t.RefreshToken,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	t, err := s.authUC.Login(r.Context(), c.Login, c.Password, time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  t.AccessToken,
		"refresh_token": t.RefreshToken,
	})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	t, err := s.authUC.Refresh(r.Context(), req.RefreshToken, time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  t.AccessToken,
		"refresh_token": t.RefreshToken,
	})
}

func (s *Server) handleOrdersSubmit(w http.ResponseWriter, r *http.Request) {
	userID, err := mw.MustUserID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	number := string(body)
	// Trim spaces/newlines
	for len(number) > 0 && (number[0] == ' ' || number[0] == '\n' || number[0] == '\r' || number[0] == '\t') {
		number = number[1:]
	}
	for len(number) > 0 && (number[len(number)-1] == ' ' || number[len(number)-1] == '\n' || number[len(number)-1] == '\r' || number[len(number)-1] == '\t') {
		number = number[:len(number)-1]
	}

	if !validate.Luhn(number) {
		http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity)
		return
	}

	res, err := s.orders.Submit(r.Context(), userID, number, time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	switch res {
	case usecase.SubmitAlreadyBySameUser:
		w.WriteHeader(http.StatusOK)
	case usecase.SubmitAlreadyByOtherUser:
		w.WriteHeader(http.StatusConflict)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

type orderResponse struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float64 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

func (s *Server) handleOrdersList(w http.ResponseWriter, r *http.Request) {
	userID, err := mw.MustUserID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	orders, err := s.orders.List(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	out := make([]orderResponse, 0, len(orders))
	for _, o := range orders {
		var accrual *float64
		if o.Accrual != nil {
			f := float64(*o.Accrual) / 100.0
			accrual = &f
		}
		out = append(out, orderResponse{
			Number:     o.Number,
			Status:     string(o.Status),
			Accrual:    accrual,
			UploadedAt: o.UploadedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	userID, err := mw.MustUserID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	b, err := s.balance.Get(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]float64{
		"current":   float64(b.Current) / 100.0,
		"withdrawn": float64(b.Withdrawn) / 100.0,
	})
}

type withdrawReq struct {
	Order string          `json:"order"`
	Sum   json.RawMessage `json:"sum"`
}

func (s *Server) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	userID, err := mw.MustUserID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req withdrawReq
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if !validate.Luhn(req.Order) {
		http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity)
		return
	}

	var sumAny any
	if err := json.Unmarshal(req.Sum, &sumAny); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	m, err := parseMoney(sumAny)
	if err != nil || m.Int64() <= 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err = s.balance.Withdraw(r.Context(), userID, req.Order, m.Int64(), time.Now())
	if err != nil {
		if err == domain.ErrInsufficientFund {
			http.Error(w, http.StatusText(http.StatusPaymentRequired), http.StatusPaymentRequired)
			return
		}
		if err == domain.ErrUnauthorized {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type withdrawalResponse struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

func (s *Server) handleWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID, err := mw.MustUserID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	ws, err := s.balance.ListWithdrawals(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(ws) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	out := make([]withdrawalResponse, 0, len(ws))
	for _, wdr := range ws {
		out = append(out, withdrawalResponse{
			Order:       wdr.OrderNumber,
			Sum:         float64(wdr.Sum) / 100.0,
			ProcessedAt: wdr.ProcessedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, out)
}
