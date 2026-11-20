package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type followUpMarkTicketRepoStub struct {
	createErr error
	created   []*Ticket
}

func (s *followUpMarkTicketRepoStub) Create(_ context.Context, ticket *Ticket, _ string) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = append(s.created, ticket)
	return nil
}
func (s *followUpMarkTicketRepoStub) GetByID(context.Context, int64) (*Ticket, error) {
	return nil, nil
}
func (s *followUpMarkTicketRepoStub) List(context.Context, pagination.PaginationParams, TicketListFilters) ([]Ticket, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *followUpMarkTicketRepoStub) UpdateStatus(context.Context, int64, string) (*Ticket, error) {
	return nil, nil
}
func (s *followUpMarkTicketRepoStub) AddMessage(context.Context, int64, *TicketMessage) error {
	return nil
}

type followUpMarkRepoStub struct {
	calls   []int64
	flagged []bool
	err     error
}

func (s *followUpMarkRepoStub) SetTicketFlag(_ context.Context, id int64, flagged bool) error {
	s.calls = append(s.calls, id)
	s.flagged = append(s.flagged, flagged)
	return s.err
}

type followUpExemptionCheckStub struct {
	invalidated []int64
}

func (s *followUpExemptionCheckStub) IsExempt(context.Context, int64) bool { return false }
func (s *followUpExemptionCheckStub) Invalidate(id int64) {
	s.invalidated = append(s.invalidated, id)
}

func validTicketInput(userID *int64) *CreateTicketInput {
	return &CreateTicketInput{
		UserID: userID, Contact: "user@example.com", Title: "need help",
		Category: TicketCategoryAccount, Content: "something broke",
	}
}

func TestTicketCreate_LoginUserMarksTicketFlagAndInvalidatesExemption(t *testing.T) {
	repo := &followUpMarkTicketRepoStub{}
	markRepo := &followUpMarkRepoStub{}
	checker := &followUpExemptionCheckStub{}
	svc := NewTicketService(repo)
	svc.SetFollowUpMarkDeps(markRepo, checker)

	userID := int64(42)
	_, err := svc.Create(context.Background(), validTicketInput(&userID))

	require.NoError(t, err)
	require.Equal(t, []int64{42}, markRepo.calls)
	require.Equal(t, []bool{true}, markRepo.flagged)
	require.Equal(t, []int64{42}, checker.invalidated, "exemption cache must be invalidated after marking")
}

func TestTicketCreate_GuestSkipsMarking(t *testing.T) {
	repo := &followUpMarkTicketRepoStub{}
	markRepo := &followUpMarkRepoStub{}
	checker := &followUpExemptionCheckStub{}
	svc := NewTicketService(repo)
	svc.SetFollowUpMarkDeps(markRepo, checker)

	_, err := svc.Create(context.Background(), validTicketInput(nil))

	require.NoError(t, err)
	require.Empty(t, markRepo.calls, "guest ticket has no user to mark")
	require.Empty(t, checker.invalidated)
}

func TestTicketCreate_MarkFailureDoesNotFailTicket(t *testing.T) {
	repo := &followUpMarkTicketRepoStub{}
	markRepo := &followUpMarkRepoStub{err: context.DeadlineExceeded}
	checker := &followUpExemptionCheckStub{}
	svc := NewTicketService(repo)
	svc.SetFollowUpMarkDeps(markRepo, checker)

	userID := int64(42)
	ticket, err := svc.Create(context.Background(), validTicketInput(&userID))

	require.NoError(t, err, "marking is best-effort and must not fail ticket creation")
	require.NotNil(t, ticket)
	require.Empty(t, checker.invalidated, "do not invalidate cache when marking failed")
}

func TestTicketCreate_CreateFailureSkipsMarking(t *testing.T) {
	repo := &followUpMarkTicketRepoStub{createErr: context.DeadlineExceeded}
	markRepo := &followUpMarkRepoStub{}
	svc := NewTicketService(repo)
	svc.SetFollowUpMarkDeps(markRepo, &followUpExemptionCheckStub{})

	userID := int64(42)
	_, err := svc.Create(context.Background(), validTicketInput(&userID))

	require.Error(t, err)
	require.Empty(t, markRepo.calls, "must not mark when ticket creation failed")
}

func TestTicketCreate_NoDepsKeepsLegacyBehavior(t *testing.T) {
	repo := &followUpMarkTicketRepoStub{}
	svc := NewTicketService(repo)

	userID := int64(42)
	ticket, err := svc.Create(context.Background(), validTicketInput(&userID))

	require.NoError(t, err)
	require.NotNil(t, ticket)
	require.Len(t, repo.created, 1)
}
