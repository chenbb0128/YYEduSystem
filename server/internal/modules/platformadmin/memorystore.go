package platformadmin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

type memoryInvite struct {
	Invite
	codeHash string
}

type MemoryStore struct {
	mu            sync.RWMutex
	nextID        uint64
	organizations []Organization
	invites       []memoryInvite
	registrations []Registration
}

func NewMemoryStore() *MemoryStore {
	now := time.Now().UTC()
	return &MemoryStore{
		nextID:        2,
		organizations: []Organization{{ID: 1, Name: "我的托管班", Slug: "default", Status: OrganizationStatusActive, CreatedAt: now, UpdatedAt: now}},
	}
}

func (s *MemoryStore) newID() uint64 { id := s.nextID; s.nextID++; return id }

func (s *MemoryStore) ListOrganizations(context.Context) ([]Organization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.organizations), nil
}

func (s *MemoryStore) CreateOrganization(_ context.Context, params CreateOrganizationParams) (Organization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name, slug := strings.TrimSpace(params.Name), strings.TrimSpace(params.Slug)
	if name == "" || slug == "" {
		return Organization{}, ErrConflict
	}
	for _, item := range s.organizations {
		if strings.EqualFold(item.Slug, slug) {
			return Organization{}, ErrConflict
		}
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = OrganizationStatusPending
	}
	now := time.Now().UTC()
	item := Organization{ID: s.newID(), Name: name, Slug: slug, ContactName: strings.TrimSpace(params.ContactName), ContactPhone: strings.TrimSpace(params.ContactPhone), AuthorizedUntil: cloneTime(params.AuthorizedUntil), Status: status, CreatedAt: now, UpdatedAt: now}
	s.organizations = append(s.organizations, item)
	return item, nil
}

func (s *MemoryStore) SetOrganizationStatus(_ context.Context, id uint64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status != OrganizationStatusPending && status != OrganizationStatusActive && status != OrganizationStatusDisabled {
		return ErrInvalidStatus
	}
	for index := range s.organizations {
		if s.organizations[index].ID == id {
			s.organizations[index].Status = status
			s.organizations[index].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return ErrNotFound
}

func (s *MemoryStore) ListInvites(_ context.Context, status string) ([]Invite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	out := make([]Invite, 0, len(s.invites))
	for index := range s.invites {
		item := &s.invites[index]
		if item.Status == InviteStatusActive && item.ExpiresAt != nil && !now.Before(*item.ExpiresAt) {
			item.Status = InviteStatusRevoked
			item.UpdatedAt = now
		}
		if strings.TrimSpace(status) != "" && item.Status != status {
			continue
		}
		out = append(out, item.Invite)
	}
	return out, nil
}

func (s *MemoryStore) CreateInvite(_ context.Context, params CreateInviteParams) (Invite, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if params.MaxUses == 0 {
		params.MaxUses = 1
	}
	code, err := NewInviteCode()
	if err != nil {
		return Invite{}, "", err
	}
	now := time.Now().UTC()
	item := Invite{ID: s.newID(), CodeHint: code[:minInt(8, len(code))], MaxUses: params.MaxUses, ExpiresAt: cloneTime(params.ExpiresAt), Status: InviteStatusActive, Note: strings.TrimSpace(params.Note), CreatedByID: params.CreatedByID, CreatedAt: now, UpdatedAt: now}
	s.invites = append(s.invites, memoryInvite{Invite: item, codeHash: HashInviteCode(code)})
	return item, code, nil
}

func (s *MemoryStore) RevokeInvite(_ context.Context, id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.invites {
		if s.invites[index].ID == id {
			if s.invites[index].Status != InviteStatusActive {
				return ErrInvalidStatus
			}
			s.invites[index].Status = InviteStatusRevoked
			s.invites[index].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return ErrNotFound
}

func (s *MemoryStore) CreateRegistration(_ context.Context, params CreateRegistrationParams) (Registration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	codeHash := HashInviteCode(params.InviteCode)
	now := time.Now().UTC()
	for index := range s.invites {
		item := &s.invites[index]
		if item.codeHash != codeHash {
			continue
		}
		if item.Status != InviteStatusActive || (item.ExpiresAt != nil && !now.Before(*item.ExpiresAt)) {
			return Registration{}, ErrInvalidInvite
		}
		if item.UsedCount >= item.MaxUses {
			item.Status = InviteStatusExhausted
			return Registration{}, ErrInviteExhausted
		}
		item.UsedCount++
		if item.UsedCount >= item.MaxUses {
			item.Status = InviteStatusExhausted
		}
		item.UpdatedAt = now
		registration := Registration{ID: s.newID(), InviteID: item.ID, OrganizationName: strings.TrimSpace(params.OrganizationName), Slug: strings.TrimSpace(params.Slug), ContactName: strings.TrimSpace(params.ContactName), ContactPhone: strings.TrimSpace(params.ContactPhone), AdminUsername: strings.TrimSpace(params.AdminUsername), AdminPasswordHash: params.AdminPasswordHash, Status: RegistrationStatusPending, CreatedAt: now, UpdatedAt: now}
		s.registrations = append(s.registrations, registration)
		return registration, nil
	}
	return Registration{}, ErrInvalidInvite
}

func (s *MemoryStore) GetRegistration(_ context.Context, id uint64) (Registration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.registrations {
		if item.ID == id {
			return cloneRegistration(item), nil
		}
	}
	return Registration{}, ErrNotFound
}

func (s *MemoryStore) ListRegistrations(_ context.Context, status string) ([]Registration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Registration, 0, len(s.registrations))
	for _, item := range s.registrations {
		if strings.TrimSpace(status) == "" || item.Status == status {
			out = append(out, cloneRegistration(item))
		}
	}
	return out, nil
}

func (s *MemoryStore) SetRegistrationStatus(_ context.Context, params SetRegistrationStatusParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if params.Status != RegistrationStatusPending && params.Status != RegistrationStatusApproved && params.Status != RegistrationStatusRejected {
		return ErrInvalidStatus
	}
	for index := range s.registrations {
		if s.registrations[index].ID == params.ID {
			item := &s.registrations[index]
			item.Status = params.Status
			item.OrganizationID = cloneID(params.OrganizationID)
			item.ReviewNote = strings.TrimSpace(params.ReviewNote)
			item.ReviewedByID = cloneIDValue(params.ReviewedByID)
			now := time.Now().UTC()
			item.ReviewedAt = &now
			item.UpdatedAt = now
			return nil
		}
	}
	return ErrNotFound
}

func NewInviteCode() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	return "DY-" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}
func HashInviteCode(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return fmt.Sprintf("%x", sum[:])
}
func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func cloneID(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func cloneIDValue(value uint64) *uint64 {
	if value == 0 {
		return nil
	}
	return &value
}
func cloneRegistration(value Registration) Registration {
	value.OrganizationID = cloneID(value.OrganizationID)
	value.ReviewedByID = cloneID(value.ReviewedByID)
	value.ReviewedAt = cloneTime(value.ReviewedAt)
	return value
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
