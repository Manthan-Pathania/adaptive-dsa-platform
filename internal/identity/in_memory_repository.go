package identity

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryRepository implements Repository for fast, deterministic unit testing.
type InMemoryRepository struct {
	mu           sync.Mutex
	users        map[uuid.UUID]*User
	usersByEmail map[string]*User
	passwords    map[uuid.UUID]string
	sessions     map[string]*Session
	auditLogs    []*AuditLog
	profiles     map[uuid.UUID]*UserProfile
}

// NewInMemoryRepository instantiates an in-memory repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		users:        make(map[uuid.UUID]*User),
		usersByEmail: make(map[string]*User),
		passwords:    make(map[uuid.UUID]string),
		sessions:     make(map[string]*Session),
		auditLogs:    make([]*AuditLog, 0),
		profiles:     make(map[uuid.UUID]*UserProfile),
	}
}

func (m *InMemoryRepository) CreateUser(ctx context.Context, email, passwordHash, fullName string, role Role) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	if _, exists := m.usersByEmail[lowerEmail]; exists {
		return nil, ErrDuplicateEmail
	}

	u := &User{
		ID:        uuid.New(),
		Email:     lowerEmail,
		FullName:  fullName,
		Role:      role,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	m.users[u.ID] = u
	m.usersByEmail[u.Email] = u
	m.passwords[u.ID] = passwordHash
	return u, nil
}

func (m *InMemoryRepository) GetUserByEmail(ctx context.Context, email string) (*User, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	u, exists := m.usersByEmail[lowerEmail]
	if !exists {
		return nil, "", ErrUserNotFound
	}
	return u, m.passwords[u.ID], nil
}

func (m *InMemoryRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, exists := m.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func (m *InMemoryRepository) UpdateUserRole(ctx context.Context, id uuid.UUID, role Role) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, exists := m.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	u.Role = role
	u.UpdatedAt = time.Now().UTC()
	return u, nil
}

func (m *InMemoryRepository) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash, ip, userAgent string, expiresAt time.Time) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := &Session{
		ID:               uuid.New(),
		UserID:           userID,
		SessionTokenHash: tokenHash,
		IPAddress:        ip,
		UserAgent:        userAgent,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now().UTC(),
		LastActiveAt:     time.Now().UTC(),
	}
	m.sessions[tokenHash] = s
	return s, nil
}

func (m *InMemoryRepository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[tokenHash]
	if !exists {
		return nil, ErrSessionNotFound
	}

	u, exists := m.users[s.UserID]
	if !exists || !u.IsActive {
		return nil, ErrSessionNotFound
	}
	s.User = u
	return s, nil
}

func (m *InMemoryRepository) TouchSession(ctx context.Context, sessionID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.sessions {
		if s.ID == sessionID {
			s.LastActiveAt = time.Now().UTC()
			return nil
		}
	}
	return nil
}

func (m *InMemoryRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, tokenHash)
	return nil
}

func (m *InMemoryRepository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for h, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, h)
		}
	}
	return nil
}

func (m *InMemoryRepository) CreateAuditLog(ctx context.Context, actorID *uuid.UUID, action, targetType string, targetID *uuid.UUID, ip, userAgent string, metadata map[string]interface{}) (*AuditLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log := &AuditLog{
		ID:          uuid.New(),
		ActorUserID: actorID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		IPAddress:   ip,
		UserAgent:   userAgent,
		Metadata:    metadata,
		CreatedAt:   time.Now().UTC(),
	}
	m.auditLogs = append(m.auditLogs, log)
	return log, nil
}

func (m *InMemoryRepository) ExecTx(ctx context.Context, fn func(repo Repository) error) error {
	m.mu.Lock()
	snapshotUsers := make(map[uuid.UUID]*User)
	for k, v := range m.users {
		u := *v
		snapshotUsers[k] = &u
	}
	snapshotUsersByEmail := make(map[string]*User)
	for k, v := range m.usersByEmail {
		u := *v
		snapshotUsersByEmail[k] = &u
	}
	snapshotPasswords := make(map[uuid.UUID]string)
	for k, v := range m.passwords {
		snapshotPasswords[k] = v
	}
	snapshotSessions := make(map[string]*Session)
	for k, v := range m.sessions {
		s := *v
		snapshotSessions[k] = &s
	}
	snapshotAuditLogs := make([]*AuditLog, len(m.auditLogs))
	copy(snapshotAuditLogs, m.auditLogs)
	snapshotProfiles := make(map[uuid.UUID]*UserProfile)
	for k, v := range m.profiles {
		p := *v
		snapshotProfiles[k] = &p
	}
	m.mu.Unlock()

	err := fn(m)
	if err != nil {
		m.mu.Lock()
		m.users = snapshotUsers
		m.usersByEmail = snapshotUsersByEmail
		m.passwords = snapshotPasswords
		m.sessions = snapshotSessions
		m.auditLogs = snapshotAuditLogs
		m.profiles = snapshotProfiles
		m.mu.Unlock()
		return err
	}
	return nil
}

func (m *InMemoryRepository) GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.profiles[userID]
	if !exists {
		return nil, ErrProfileNotFound
	}
	cp := *p
	return &cp, nil
}

func (m *InMemoryRepository) UpsertUserProfile(ctx context.Context, profile *UserProfile) (*UserProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	cp := *profile
	if cp.CreatedAt.IsZero() {
		if existing, ok := m.profiles[cp.UserID]; ok {
			cp.CreatedAt = existing.CreatedAt
		} else {
			cp.CreatedAt = now
		}
	}
	cp.UpdatedAt = now
	m.profiles[cp.UserID] = &cp
	res := cp
	return &res, nil
}
