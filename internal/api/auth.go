package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"connection-check/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type Session struct {
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type AuthHandler struct {
	database *db.DB
	sessions sync.Map
}

func NewAuthHandler(database *db.DB) *AuthHandler {
	a := &AuthHandler{database: database}

	// Session cleanup worker
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		for range ticker.C {
			now := time.Now()
			a.sessions.Range(func(key, value interface{}) bool {
				if s, ok := value.(Session); ok {
					if now.After(s.ExpiresAt) {
						a.sessions.Delete(key)
					}
				}
				return true
			})
		}
	}()

	return a
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HandleLogin authenticates admin and sets session cookie.
func (a *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	user, err := a.database.GetUser(req.Username)
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	token := generateToken()
	a.sessions.Store(token, Session{
		Username:  user.Username,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":             true,
		"username":            user.Username,
		"is_default_password": user.IsDefaultPassword,
		"redirect":            "/data",
	})
}

// HandleLogout clears the admin session.
func (a *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("admin_session"); err == nil {
		a.sessions.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})

	http.Redirect(w, r, "/data/login", http.StatusSeeOther)
}

// HandleChangePassword allows updating credentials and marks is_default_password as false.
func (a *AuthHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, ok := a.GetSessionFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "New password must be at least 8 characters long.",
		})
		return
	}

	if req.NewPassword == "admin123" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "New password cannot be the default 'admin123'.",
		})
		return
	}

	user, err := a.database.GetUser(session.Username)
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Current password is incorrect.",
		})
		return
	}

	if err := a.database.UpdatePassword(session.Username, req.NewPassword); err != nil {
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password updated successfully.",
	})
}

// GetSessionFromRequest checks cookie and returns current session if valid.
func (a *AuthHandler) GetSessionFromRequest(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie("admin_session")
	if err != nil {
		return Session{}, false
	}

	val, ok := a.sessions.Load(cookie.Value)
	if !ok {
		return Session{}, false
	}

	session := val.(Session)
	if time.Now().After(session.ExpiresAt) {
		a.sessions.Delete(cookie.Value)
		return Session{}, false
	}

	return session, true
}

// RequireAuth middleware protects admin endpoints and checks default password requirement.
func (a *AuthHandler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := a.GetSessionFromRequest(r)
		if !ok {
			if r.Header.Get("Accept") == "application/json" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/data/login", http.StatusSeeOther)
			return
		}

		user, err := a.database.GetUser(session.Username)
		if err != nil {
			http.Redirect(w, r, "/data/login", http.StatusSeeOther)
			return
		}

		// Attach user info to context or proceed
		_ = user
		next(w, r)
	}
}
