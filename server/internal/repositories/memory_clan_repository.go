package repositories

import (
	"context"
	"strings"
	"sync"

	"go-service-template/internal/models"
)

type MemoryClanRepository struct {
	mu    sync.Mutex
	clans map[string]models.Clan
}

func NewMemoryClanRepository() *MemoryClanRepository {
	return &MemoryClanRepository{clans: make(map[string]models.Clan)}
}

func (r *MemoryClanRepository) Find(_ context.Context, name string) (models.Clan, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	clan, ok := r.clans[name]
	return clan, ok, nil
}

func (r *MemoryClanRepository) Create(_ context.Context, clan models.Clan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clans[clan.Name]; ok {
		return ErrClanExists
	}
	r.clans[clan.Name] = clan
	return nil
}

func (r *MemoryClanRepository) FindByMark(_ context.Context, mark [3]int32, excludeName string) (models.Clan, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, clan := range r.clans {
		if clan.Name != excludeName && clan.Mark == mark {
			return clan, true, nil
		}
	}
	return models.Clan{}, false, nil
}

func (r *MemoryClanRepository) SaveMark(_ context.Context, name string, mark [3]int32, markExtra uint16) error {
	return r.update(name, func(clan *models.Clan) {
		clan.Mark = mark
		clan.MarkExtra = markExtra
	})
}

func (r *MemoryClanRepository) Delete(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clans, name)
	return nil
}

func (r *MemoryClanRepository) SearchByName(_ context.Context, name string, limit int64) ([]models.Clan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	prefix := strings.ToLower(name)
	var found []models.Clan
	for _, clan := range r.clans {
		if int64(len(found)) == limit {
			break
		}
		if strings.HasPrefix(strings.ToLower(clan.Name), prefix) {
			found = append(found, clan)
		}
	}
	return found, nil
}

func (r *MemoryClanRepository) FindByApplicant(_ context.Context, steamID string) (models.Clan, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, clan := range r.clans {
		for _, applicant := range clan.Applicants {
			if applicant.SteamID == steamID {
				return clan, true, nil
			}
		}
	}
	return models.Clan{}, false, nil
}

func (r *MemoryClanRepository) SaveApplicants(_ context.Context, name string, applicants []models.ClanApplicant) error {
	return r.update(name, func(clan *models.Clan) {
		clan.Applicants = applicants
	})
}

func (r *MemoryClanRepository) SaveMembers(_ context.Context, name string, members []models.ClanMember) error {
	return r.update(name, func(clan *models.Clan) {
		clan.Members = members
	})
}

func (r *MemoryClanRepository) SaveRoster(_ context.Context, name string, members []models.ClanMember, applicants []models.ClanApplicant) error {
	return r.update(name, func(clan *models.Clan) {
		clan.Members = members
		clan.Applicants = applicants
	})
}

func (r *MemoryClanRepository) SaveLeadership(_ context.Context, name, master string, members []models.ClanMember) error {
	return r.update(name, func(clan *models.Clan) {
		clan.Master = master
		clan.Members = members
	})
}

func (r *MemoryClanRepository) update(name string, apply func(*models.Clan)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	clan, ok := r.clans[name]
	if !ok {
		return nil
	}
	apply(&clan)
	r.clans[name] = clan
	return nil
}
