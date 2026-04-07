package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"

	"repairplanner/internal/dto"
	"repairplanner/internal/repository"
)

type Server struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Server {
	return &Server{repo: repo}
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	errCh := make(chan error, 1)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
					errCh <- nil
					return
				}
				errCh <- err
				return
			}
			go s.handleConn(conn)
		}
	}()

	return <-errCh
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		var req dto.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = writeJSON(conn, dto.Response{
				OK:     false,
				Action: "",
				Error:  "invalid json: " + err.Error(),
			})
			continue
		}

		resp := s.dispatch(req)
		_ = writeJSON(conn, resp)
	}
}

func (s *Server) dispatch(req dto.Request) dto.Response {
	ctx, cancel := context.WithTimeout(context.Background(), 20_000_000_000)
	defer cancel()

	switch req.Action {
	case "ping":
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"message": "pong"}}

	// ---- lists ----
	case "machines.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM machines ORDER BY machine_id`)
		return wrap(req.Action, rows, err)

	case "machine_events.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM machine_events ORDER BY event_date DESC, event_id DESC`)
		return wrap(req.Action, rows, err)

	case "repair_acts.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_acts ORDER BY COALESCE(end_date, start_date) DESC NULLS LAST, act_id DESC`)
		return wrap(req.Action, rows, err)

	case "repair_requests.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_requests ORDER BY priority_weight DESC, request_id DESC`)
		return wrap(req.Action, rows, err)

	case "repair_tech_cards.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_tech_cards ORDER BY repair_type, model`)
		return wrap(req.Action, rows, err)

	case "monthly_resources.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM monthly_resources ORDER BY month`)
		return wrap(req.Action, rows, err)

	case "repair_plan.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_plan ORDER BY assigned_month, request_id`)
		return wrap(req.Action, rows, err)

	case "material_norms.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM material_norms ORDER BY repair_type, model, material_code`)
		return wrap(req.Action, rows, err)

	case "brigades.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM brigades ORDER BY brigade_number`)
		return wrap(req.Action, rows, err)

	case "repair_assignments.list":
		rows, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_assignments ORDER BY assignment_id DESC`)
		return wrap(req.Action, rows, err)

	// ---- inputs ----
	case "machines.create":
		var in dto.MachineInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		id, err := s.repo.CreateMachine(ctx, in)
		if err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"machine_id": id}}

	case "machine_events.create":
		var in dto.MachineEventInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		id, err := s.repo.CreateMachineEvent(ctx, in)
		if err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"event_id": id}}

	case "repair_acts.create":
		var in dto.RepairActInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		id, err := s.repo.CreateRepairAct(ctx, in)
		if err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"act_id": id}}

	case "repair_requests.create":
		var in dto.RepairRequestInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		id, err := s.repo.CreateRepairRequest(ctx, in)
		if err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"request_id": id}}

	case "repair_tech_cards.upsert":
		var in dto.TechCardInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		id, err := s.repo.UpsertTechCard(ctx, in)
		if err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"tech_card_id": id}}

	case "monthly_resources.upsert":
		var in dto.MonthlyResourceInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		if err := s.repo.UpsertMonthlyResource(ctx, in); err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"saved": true}}

	case "repair_plan.upsert":
		var in dto.RepairPlanInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		id, err := s.repo.UpsertRepairPlan(ctx, in)
		if err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"plan_id": id}}

	case "material_norms.upsert":
		var in dto.MaterialNormInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		id, err := s.repo.UpsertMaterialNorm(ctx, in)
		if err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"norm_id": id}}

	case "brigades.upsert":
		var in dto.BrigadeInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		if err := s.repo.UpsertBrigade(ctx, in); err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"saved": true}}

	case "repair_assignments.upsert":
		var in dto.AssignmentInput
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		id, err := s.repo.UpsertRepairAssignment(ctx, in)
		if err != nil {
			return bad(req.Action, err)
		}
		return dto.Response{OK: true, Action: req.Action, Data: map[string]any{"assignment_id": id}}

	// ---- tasks ----
	case "reports.current_registry":
		rows, err := s.repo.CurrentRegistry(ctx)
		return wrap(req.Action, rows, err)

	case "reports.material_demand":
		var in struct {
			Month string `json:"month"`
		}
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		rows, err := s.repo.MaterialDemand(ctx, in.Month)
		return wrap(req.Action, rows, err)

	case "algorithms.year_plan":
		var in struct {
			Year int `json:"year"`
		}
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		rows, err := s.repo.GenerateYearPlan(ctx, in.Year)
		return wrap(req.Action, rows, err)

	case "algorithms.assign_brigades":
		var in struct {
			Month string `json:"month"`
		}
		if err := json.Unmarshal(req.Data, &in); err != nil {
			return bad(req.Action, err)
		}
		rows, err := s.repo.AssignBrigades(ctx, in.Month)
		return wrap(req.Action, rows, err)

	case "snapshot.get":
		machines, err := s.repo.QueryAll(ctx, `SELECT * FROM machines ORDER BY machine_id`)
		if err != nil { return bad(req.Action, err) }

		events, err := s.repo.QueryAll(ctx, `SELECT * FROM machine_events ORDER BY event_date DESC, event_id DESC`)
		if err != nil { return bad(req.Action, err) }

		acts, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_acts ORDER BY COALESCE(end_date, start_date) DESC NULLS LAST, act_id DESC`)
		if err != nil { return bad(req.Action, err) }

		requests, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_requests ORDER BY priority_weight DESC, request_id DESC`)
		if err != nil { return bad(req.Action, err) }

		techCards, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_tech_cards ORDER BY repair_type, model`)
		if err != nil { return bad(req.Action, err) }

		resources, err := s.repo.QueryAll(ctx, `SELECT * FROM monthly_resources ORDER BY month`)
		if err != nil { return bad(req.Action, err) }

		plan, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_plan ORDER BY assigned_month, request_id`)
		if err != nil { return bad(req.Action, err) }

		norms, err := s.repo.QueryAll(ctx, `SELECT * FROM material_norms ORDER BY repair_type, model, material_code`)
		if err != nil { return bad(req.Action, err) }

		brigades, err := s.repo.QueryAll(ctx, `SELECT * FROM brigades ORDER BY brigade_number`)
		if err != nil { return bad(req.Action, err) }

		assignments, err := s.repo.QueryAll(ctx, `SELECT * FROM repair_assignments ORDER BY assignment_id DESC`)
		if err != nil { return bad(req.Action, err) }

		registry, err := s.repo.CurrentRegistry(ctx)
		if err != nil { return bad(req.Action, err) }

		return dto.Response{
			OK:     true,
			Action: req.Action,
			Data: map[string]any{
				"machines":          machines,
				"machine_events":    events,
				"repair_acts":       acts,
				"repair_requests":   requests,
				"repair_tech_cards": techCards,
				"monthly_resources": resources,
				"repair_plan":       plan,
				"material_norms":    norms,
				"brigades":          brigades,
				"repair_assignments": assignments,
				"current_registry":  registry,
			},
		}

	default:
		return dto.Response{OK: false, Action: req.Action, Error: "unknown action"}
	}
}

func wrap(action string, rows []map[string]any, err error) dto.Response {
	if err != nil {
		return bad(action, err)
	}
	return dto.Response{OK: true, Action: action, Data: rows}
}

func bad(action string, err error) dto.Response {
	return dto.Response{OK: false, Action: action, Error: err.Error()}
}

func writeJSON(conn net.Conn, v any) error {
	return json.NewEncoder(conn).Encode(v)
}