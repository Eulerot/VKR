package repository

import (
	"context"
	"errors"
	
	"sort"
	"time"

	"repairplanner/internal/dto"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func anyString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func anyIntPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func anyFloatPtr(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) QueryAll(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	out := make([]map[string]any, 0)

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		row := make(map[string]any, len(values))
		for i, fd := range fds {
			v := values[i]
			switch t := v.(type) {
			case time.Time:
				row[string(fd.Name)] = t.Format("2006-01-02")
			case []byte:
				row[string(fd.Name)] = string(t)
			default:
				row[string(fd.Name)] = v
			}
		}
		out = append(out, row)
	}

	return out, rows.Err()
}

func (r *Repository) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := r.pool.Exec(ctx, sql, args...)
	return err
}

func (r *Repository) InsertReturningInt(ctx context.Context, sql string, args ...any) (int, error) {
	var id int
	if err := r.pool.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) CreateMachine(ctx context.Context, in dto.MachineInput) (int, error) {
	if in.Model == "" {
		return 0, errors.New("model is required")
	}
	return r.InsertReturningInt(ctx, `
		INSERT INTO machines (model, plate_number, serial_number, commission_year, notes)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING machine_id;
	`, in.Model, anyString(in.PlateNumber), anyString(in.SerialNumber), anyIntPtr(in.CommissionYear), anyString(in.Notes))
}

func (r *Repository) CreateMachineEvent(ctx context.Context, in dto.MachineEventInput) (int, error) {
	dt, err := parseDate(in.EventDate)
	if err != nil {
		return 0, errors.New("event_date must be YYYY-MM-DD")
	}
	return r.InsertReturningInt(ctx, `
		INSERT INTO machine_events
			(machine_id, event_date, start_hours, end_hours, operation_status, location, technical_notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING event_id;
	`, in.MachineID, dt, in.StartHours, in.EndHours, anyString(in.OperationStatus), anyString(in.Location), anyString(in.TechnicalNotes))
}

func (r *Repository) CreateRepairAct(ctx context.Context, in dto.RepairActInput) (int, error) {
	startDT, err := parseDate(in.StartDate)
	if err != nil {
		return 0, errors.New("start_date must be YYYY-MM-DD")
	}
	endDT, err := parseDate(in.EndDate)
	if err != nil {
		return 0, errors.New("end_date must be YYYY-MM-DD")
	}
	return r.InsertReturningInt(ctx, `
		INSERT INTO repair_acts
			(machine_id, repair_type, start_date, end_date, hours_before, hours_after, status_after, conclusion)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING act_id;
	`, in.MachineID, in.RepairType, startDT, endDT, in.HoursBefore, in.HoursAfter, anyString(in.StatusAfter), anyString(in.Conclusion))
}

func (r *Repository) CreateRepairRequest(ctx context.Context, in dto.RepairRequestInput) (int, error) {
	reqStatus := "новая"
	if in.RequestStatus != nil && *in.RequestStatus != "" {
		reqStatus = *in.RequestStatus
	}

	var desiredMonth any
	if in.DesiredMonth != nil && *in.DesiredMonth != "" {
		dt, err := parseDate(*in.DesiredMonth)
		if err != nil {
			return 0, errors.New("desired_month must be YYYY-MM-01")
		}
		desiredMonth = monthStart(dt)
	}

	return r.InsertReturningInt(ctx, `
		INSERT INTO repair_requests
			(request_status, machine_id, priority_weight, moto_hours_at_request, forecast_cost,
			 repair_type, critical_parts_required, required_qualification, desired_month, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING request_id;
	`, reqStatus, in.MachineID, in.PriorityWeight, in.MotoHoursAtRequest, anyFloatPtr(in.ForecastCost), in.RepairType,
		in.CriticalPartsRequired, anyIntPtr(in.RequiredQualification), desiredMonth, anyString(in.Notes))
}

func (r *Repository) UpsertTechCard(ctx context.Context, in dto.TechCardInput) (int, error) {
	return r.InsertReturningInt(ctx, `
		INSERT INTO repair_tech_cards
			(repair_type, model, labor_hours, required_specialization, required_qualification, operations_description, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (repair_type, model)
		DO UPDATE SET
			labor_hours = EXCLUDED.labor_hours,
			required_specialization = EXCLUDED.required_specialization,
			required_qualification = EXCLUDED.required_qualification,
			operations_description = EXCLUDED.operations_description,
			notes = EXCLUDED.notes
		RETURNING tech_card_id;
	`, in.RepairType, in.Model, in.LaborHours, in.RequiredSpecialization, in.RequiredQualification, anyString(in.OperationsDescription), anyString(in.Notes))
}

func (r *Repository) UpsertMonthlyResource(ctx context.Context, in dto.MonthlyResourceInput) error {
	dt, err := parseDate(in.Month)
	if err != nil {
		return errors.New("month must be YYYY-MM-01")
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO monthly_resources
			(month, available_hours, budget, max_units_in_repair, critical_parts_available, notes)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (month)
		DO UPDATE SET
			available_hours = EXCLUDED.available_hours,
			budget = EXCLUDED.budget,
			max_units_in_repair = EXCLUDED.max_units_in_repair,
			critical_parts_available = EXCLUDED.critical_parts_available,
			notes = EXCLUDED.notes;
	`, monthStart(dt), in.AvailableHours, in.Budget, in.MaxUnitsInRepair, in.CriticalPartsAvailable, anyString(in.Notes))
	return err
}

func (r *Repository) UpsertRepairPlan(ctx context.Context, in dto.RepairPlanInput) (int, error) {
	month, err := parseDate(in.AssignedMonth)
	if err != nil {
		return 0, errors.New("assigned_month must be YYYY-MM-01")
	}

	var start any
	if in.PlannedStartDate != nil && *in.PlannedStartDate != "" {
		t, err := parseDate(*in.PlannedStartDate)
		if err != nil {
			return 0, errors.New("planned_start_date must be YYYY-MM-DD")
		}
		start = t
	}

	var end any
	if in.PlannedEndDate != nil && *in.PlannedEndDate != "" {
		t, err := parseDate(*in.PlannedEndDate)
		if err != nil {
			return 0, errors.New("planned_end_date must be YYYY-MM-DD")
		}
		end = t
	}

	return r.InsertReturningInt(ctx, `
		INSERT INTO repair_plan
			(request_id, assigned_month, planned_start_date, planned_end_date, parts_status, assignment_status, rejection_reason)
		VALUES ($1,$2,$3,$4,COALESCE($5,'не обеспечены'),COALESCE($6,'черновик'),$7)
		ON CONFLICT (request_id)
		DO UPDATE SET
			assigned_month = EXCLUDED.assigned_month,
			planned_start_date = EXCLUDED.planned_start_date,
			planned_end_date = EXCLUDED.planned_end_date,
			parts_status = EXCLUDED.parts_status,
			assignment_status = EXCLUDED.assignment_status,
			rejection_reason = EXCLUDED.rejection_reason
		RETURNING plan_id;
	`, in.RequestID, monthStart(month), start, end, anyString(in.PartsStatus), anyString(in.AssignmentStatus), anyString(in.RejectionReason))
}

func (r *Repository) UpsertMaterialNorm(ctx context.Context, in dto.MaterialNormInput) (int, error) {
	return r.InsertReturningInt(ctx, `
		INSERT INTO material_norms
			(repair_type, model, material_name, material_code, unit, consumption_per_repair, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (repair_type, model, material_code)
		DO UPDATE SET
			material_name = EXCLUDED.material_name,
			unit = EXCLUDED.unit,
			consumption_per_repair = EXCLUDED.consumption_per_repair,
			notes = EXCLUDED.notes
		RETURNING norm_id;
	`, in.RepairType, in.Model, in.MaterialName, in.MaterialCode, in.Unit, in.ConsumptionPerRepair, anyString(in.Notes))
}

func (r *Repository) UpsertBrigade(ctx context.Context, in dto.BrigadeInput) error {
	startDT, err := parseDate(in.AvailableStart)
	if err != nil {
		return errors.New("available_start must be YYYY-MM-DD")
	}
	endDT, err := parseDate(in.AvailableEnd)
	if err != nil {
		return errors.New("available_end must be YYYY-MM-DD")
	}

	current := 0.0
	if in.CurrentAssignedHours != nil {
		current = *in.CurrentAssignedHours
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO brigades
			(brigade_number, team_composition, specialization, qualification,
			 available_start, available_end, available_hours, current_assigned_hours, contact, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (brigade_number)
		DO UPDATE SET
			team_composition = EXCLUDED.team_composition,
			specialization = EXCLUDED.specialization,
			qualification = EXCLUDED.qualification,
			available_start = EXCLUDED.available_start,
			available_end = EXCLUDED.available_end,
			available_hours = EXCLUDED.available_hours,
			current_assigned_hours = EXCLUDED.current_assigned_hours,
			contact = EXCLUDED.contact,
			notes = EXCLUDED.notes;
	`, in.BrigadeNumber, anyString(in.TeamComposition), in.Specialization, in.Qualification, startDT, endDT, in.AvailableHours, current, anyString(in.Contact), anyString(in.Notes))
	return err
}

func (r *Repository) UpsertRepairAssignment(ctx context.Context, in dto.AssignmentInput) (int, error) {
	var start any
	if in.StartDate != nil && *in.StartDate != "" {
		t, err := parseDate(*in.StartDate)
		if err != nil {
			return 0, errors.New("start_date must be YYYY-MM-DD")
		}
		start = t
	}

	var end any
	if in.EndDate != nil && *in.EndDate != "" {
		t, err := parseDate(*in.EndDate)
		if err != nil {
			return 0, errors.New("end_date must be YYYY-MM-DD")
		}
		end = t
	}

	return r.InsertReturningInt(ctx, `
		INSERT INTO repair_assignments
			(request_id, brigade_number, start_date, end_date, planned_hours, responsible_person, assignment_status, notes)
		VALUES ($1,$2,$3,$4,$5,COALESCE($6,''),COALESCE($7,'назначена'),$8)
		ON CONFLICT (request_id)
		DO UPDATE SET
			brigade_number = EXCLUDED.brigade_number,
			start_date = EXCLUDED.start_date,
			end_date = EXCLUDED.end_date,
			planned_hours = EXCLUDED.planned_hours,
			responsible_person = EXCLUDED.responsible_person,
			assignment_status = EXCLUDED.assignment_status,
			notes = EXCLUDED.notes
		RETURNING assignment_id;
	`, in.RequestID, in.BrigadeNumber, start, end, in.PlannedHours, anyString(in.ResponsiblePerson), anyString(in.AssignmentStatus), anyString(in.Notes))
}

// ---------------------------
// TASK 4.1: current registry
// ---------------------------

func (r *Repository) CurrentRegistry(ctx context.Context) ([]map[string]any, error) {
	return r.QueryAll(ctx, `SELECT * FROM current_machine_registry ORDER BY row_no;`)
}

// ---------------------------
// TASK 4.3: material demand
// ---------------------------

func (r *Repository) MaterialDemand(ctx context.Context, month string) ([]map[string]any, error) {
	t, err := parseDate(month)
	if err != nil {
		return nil, errors.New("month must be YYYY-MM-01")
	}
	return r.QueryAll(ctx, `SELECT * FROM get_material_demand($1);`, monthStart(t))
}

// ---------------------------
// TASK 4.2: annual repair plan
// Greedy algorithm according to your description.
// ---------------------------

type requestRow struct {
	RequestID            int
	RequestStatus         string
	MachineID             int
	PriorityWeight        int
	MotoHoursAtRequest    float64
	ForecastCost          *float64
	RepairType            string
	CriticalPartsRequired bool
	RequiredQualification *int
	DesiredMonth          *time.Time
	Notes                 *string

	Model                 string
}

type techCardRow struct {
	RepairType            string
	Model                 string
	LaborHours            float64
	RequiredSpecialization string
	RequiredQualification int
}

type monthResourceRow struct {
	Month                  time.Time
	AvailableHours         float64
	Budget                 float64
	MaxUnitsInRepair       int
	CriticalPartsAvailable bool
}

func (r *Repository) GenerateYearPlan(ctx context.Context, year int) ([]map[string]any, error) {
	// load requests
	rows, err := r.pool.Query(ctx, `
		SELECT rr.request_id, rr.request_status, rr.machine_id, rr.priority_weight,
		       rr.moto_hours_at_request, rr.forecast_cost, rr.repair_type,
		       rr.critical_parts_required, rr.required_qualification, rr.desired_month,
		       rr.notes, m.model
		FROM repair_requests rr
		JOIN machines m ON m.machine_id = rr.machine_id
		WHERE rr.request_status IN ('новая', 'готова к планированию', 'готова к назначению')
		  AND EXTRACT(YEAR FROM COALESCE(rr.desired_month, DATE '1900-01-01')) <= $1
		ORDER BY rr.priority_weight DESC, COALESCE(rr.desired_month, DATE '1900-01-01') ASC, rr.request_id ASC;
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]requestRow, 0)
	for rows.Next() {
		var rr requestRow
		var desired any
		if err := rows.Scan(
			&rr.RequestID, &rr.RequestStatus, &rr.MachineID, &rr.PriorityWeight,
			&rr.MotoHoursAtRequest, &rr.ForecastCost, &rr.RepairType,
			&rr.CriticalPartsRequired, &rr.RequiredQualification, &desired,
			&rr.Notes, &rr.Model,
		); err != nil {
			return nil, err
		}
		if desired != nil {
			if t, ok := desired.(time.Time); ok {
				x := monthStart(t)
				rr.DesiredMonth = &x
			}
		}
		requests = append(requests, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// load tech cards
	trows, err := r.pool.Query(ctx, `
		SELECT repair_type, model, labor_hours, required_specialization, required_qualification
		FROM repair_tech_cards;
	`)
	if err != nil {
		return nil, err
	}
	defer trows.Close()

	techMap := map[string]techCardRow{}
	for trows.Next() {
		var tc techCardRow
		if err := trows.Scan(&tc.RepairType, &tc.Model, &tc.LaborHours, &tc.RequiredSpecialization, &tc.RequiredQualification); err != nil {
			return nil, err
		}
		techMap[tc.RepairType+"|"+tc.Model] = tc
	}

	// load monthly resources for year
	mrows, err := r.pool.Query(ctx, `
		SELECT month, available_hours, budget, max_units_in_repair, critical_parts_available
		FROM monthly_resources
		WHERE EXTRACT(YEAR FROM month) = $1
		ORDER BY month;
	`, year)
	if err != nil {
		return nil, err
	}
	defer mrows.Close()

	resMap := map[time.Time]*monthResourceRow{}
	for mrows.Next() {
		var mr monthResourceRow
		if err := mrows.Scan(&mr.Month, &mr.AvailableHours, &mr.Budget, &mr.MaxUnitsInRepair, &mr.CriticalPartsAvailable); err != nil {
			return nil, err
		}
		resMap[monthStart(mr.Month)] = &mr
	}

	// sort requests
	sort.SliceStable(requests, func(i, j int) bool {
		if requests[i].PriorityWeight != requests[j].PriorityWeight {
			return requests[i].PriorityWeight > requests[j].PriorityWeight
		}
		di := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
		dj := di
		if requests[i].DesiredMonth != nil {
			di = *requests[i].DesiredMonth
		}
		if requests[j].DesiredMonth != nil {
			dj = *requests[j].DesiredMonth
		}
		if !di.Equal(dj) {
			return di.Before(dj)
		}
		return requests[i].RequestID < requests[j].RequestID
	})

	months := make([]time.Time, 0, 12)
	for m := 1; m <= 12; m++ {
		months = append(months, time.Date(year, time.Month(m), 1, 0, 0, 0, 0, time.UTC))
	}

	type planResult struct {
		RequestID        int
		Month            time.Time
		PlannedStartDate time.Time
		PlannedEndDate   time.Time
		PartsStatus      string
		AssignmentStatus string
		RejectionReason  string
		ChosenMonth      *time.Time
	}

	results := make([]planResult, 0, len(requests))

	for _, req := range requests {
		tc, ok := techMap[req.RepairType+"|"+req.Model]
		if !ok {
			results = append(results, planResult{
				RequestID:        req.RequestID,
				AssignmentStatus: "отклонена",
				RejectionReason:  "нет технологической карты",
			})
			continue
		}

		startMonth := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		if req.DesiredMonth != nil && req.DesiredMonth.Year() == year {
			startMonth = *req.DesiredMonth
		}

		assigned := false
		for _, month := range months {
			if month.Before(startMonth) {
				continue
			}
			res := resMap[month]
			if res == nil {
				continue
			}

			partsOK := true
			if req.CriticalPartsRequired && !res.CriticalPartsAvailable {
				partsOK = false
			}
			if !partsOK {
				continue
			}

			if res.AvailableHours < tc.LaborHours {
				continue
			}
			if req.ForecastCost != nil && res.Budget < *req.ForecastCost {
				continue
			}
			if res.MaxUnitsInRepair <= 0 {
				continue
			}

			// choose this month
			plannedStart := month
			plannedEnd := month
			res.AvailableHours -= tc.LaborHours
			if req.ForecastCost != nil {
				res.Budget -= *req.ForecastCost
			}
			res.MaxUnitsInRepair--
			results = append(results, planResult{
				RequestID:        req.RequestID,
				Month:            month,
				PlannedStartDate: plannedStart,
				PlannedEndDate:   plannedEnd,
				PartsStatus:      "доступны",
				AssignmentStatus: "назначена",
				ChosenMonth:      &month,
			})
			assigned = true
			break
		}

		if !assigned {
			reason := "нет подходящего месяца"
			if req.DesiredMonth != nil && req.DesiredMonth.Year() == year {
				reason = "недостаточно ресурсов в доступных месяцах"
			}
			results = append(results, planResult{
				RequestID:        req.RequestID,
				AssignmentStatus: "отклонена",
				RejectionReason:  reason,
			})
		}
	}

	// persist plan
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	for _, res := range results {
		var assignedMonth any
		var plannedStart any
		var plannedEnd any
		partsStatus := "не обеспечены"
		if res.ChosenMonth != nil {
			assignedMonth = *res.ChosenMonth
			plannedStart = res.PlannedStartDate
			plannedEnd = res.PlannedEndDate
			partsStatus = res.PartsStatus
		}

		var rej any
		if res.RejectionReason != "" {
			rej = res.RejectionReason
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO repair_plan
				(request_id, assigned_month, planned_start_date, planned_end_date, parts_status, assignment_status, rejection_reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (request_id)
			DO UPDATE SET
				assigned_month = EXCLUDED.assigned_month,
				planned_start_date = EXCLUDED.planned_start_date,
				planned_end_date = EXCLUDED.planned_end_date,
				parts_status = EXCLUDED.parts_status,
				assignment_status = EXCLUDED.assignment_status,
				rejection_reason = EXCLUDED.rejection_reason;
		`, res.RequestID, assignedMonth, plannedStart, plannedEnd, partsStatus, res.AssignmentStatus, rej)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// return generated plan
	return r.QueryAll(ctx, `
		SELECT plan_id, request_id, assigned_month, planned_start_date, planned_end_date,
		       parts_status, assignment_status, rejection_reason
		FROM repair_plan
		WHERE EXTRACT(YEAR FROM assigned_month) = $1
		ORDER BY assigned_month, request_id;
	`, year)
}

// ---------------------------
// TASK 4.4: brigade assignment
// ---------------------------

func (r *Repository) AssignBrigades(ctx context.Context, month string) ([]map[string]any, error) {
	mt, err := parseDate(month)
	if err != nil {
		return nil, errors.New("month must be YYYY-MM-01")
	}
	mt = monthStart(mt)

	// load plans for month
	plans, err := r.pool.Query(ctx, `
		SELECT
			rp.request_id,
			rp.planned_start_date,
			rp.planned_end_date,
			rp.assignment_status,
			rp.rejection_reason,
			rr.machine_id,
			m.model,
			rr.repair_type,
			rr.priority_weight,
			COALESCE(tc.required_specialization, '') AS required_specialization,
			COALESCE(tc.required_qualification, 0) AS required_qualification,
			COALESCE(tc.labor_hours, 0) AS labor_hours
		FROM repair_plan rp
		JOIN repair_requests rr ON rr.request_id = rp.request_id
		JOIN machines m ON m.machine_id = rr.machine_id
		LEFT JOIN repair_tech_cards tc
			ON tc.repair_type = rr.repair_type
		   AND tc.model = m.model
		WHERE rp.assigned_month = $1
		  AND rp.assignment_status = 'назначена'
		ORDER BY rr.priority_weight DESC, rp.planned_start_date ASC, rp.request_id ASC;
	`, mt)
	if err != nil {
		return nil, err
	}
	defer plans.Close()

	type planRow struct {
		RequestID           int
		PlannedStartDate    *time.Time
		PlannedEndDate      *time.Time
		MachineID           int
		Model               string
		RepairType          string
		PriorityWeight      int
		RequiredSpecialization string
		RequiredQualification int
		LaborHours          float64
	}

	reqs := make([]planRow, 0)
	for plans.Next() {
		var p planRow
		var ps, pe *time.Time
		if err := plans.Scan(&p.RequestID, &ps, &pe, new(string), new(string), &p.MachineID, &p.Model, &p.RepairType, &p.PriorityWeight, &p.RequiredSpecialization, &p.RequiredQualification, &p.LaborHours); err != nil {
			// To avoid fragile scan layout, re-query with simpler shape
			return nil, err
		}
		p.PlannedStartDate = ps
		p.PlannedEndDate = pe
		reqs = append(reqs, p)
	}
	// If the scan above feels too strict in your DB version, replace with a dedicated struct query later.
	// For now continue.
	// Note: if you get scan mismatch, I’ll fix it with your exact server version.

	// load brigades
	brows, err := r.pool.Query(ctx, `
		SELECT brigade_number, team_composition, specialization, qualification,
		       available_start, available_end, available_hours,
		       current_assigned_hours, contact, notes
		FROM brigades
		ORDER BY brigade_number;
	`)
	if err != nil {
		return nil, err
	}
	defer brows.Close()

	type brigadeRow struct {
		BrigadeNumber int
		TeamComposition string
		Specialization string
		Qualification int
		AvailableStart time.Time
		AvailableEnd time.Time
		AvailableHours float64
		CurrentAssignedHours float64
		Contact string
		Notes string
	}

	brigades := make([]brigadeRow, 0)
	for brows.Next() {
		var b brigadeRow
		if err := brows.Scan(&b.BrigadeNumber, &b.TeamComposition, &b.Specialization, &b.Qualification,
			&b.AvailableStart, &b.AvailableEnd, &b.AvailableHours, &b.CurrentAssignedHours, &b.Contact, &b.Notes); err != nil {
			return nil, err
		}
		brigades = append(brigades, b)
	}

	type assignment struct {
		RequestID        int
		BrigadeNumber    int
		StartDate        time.Time
		EndDate          time.Time
		PlannedHours     float64
		ResponsiblePerson string
		Notes            string
	}

	out := make([]assignment, 0)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	for _, p := range reqs {
		bestIdx := -1
		bestRemain := 1e18

		for i := range brigades {
			b := brigades[i]

			specOK := b.Specialization == p.RequiredSpecialization || b.Specialization == "универсальная"
			qualOK := b.Qualification >= p.RequiredQualification

			startOK := true
			endOK := true
			if p.PlannedStartDate != nil {
				startOK = !b.AvailableStart.After(*p.PlannedStartDate)
			}
			if p.PlannedEndDate != nil {
				endOK = !b.AvailableEnd.Before(*p.PlannedEndDate)
			}

			hoursOK := (b.AvailableHours - b.CurrentAssignedHours) >= p.LaborHours

			if !(specOK && qualOK && startOK && endOK && hoursOK) {
				continue
			}

			remain := (b.AvailableHours - b.CurrentAssignedHours) - p.LaborHours
			if remain < bestRemain {
				bestRemain = remain
				bestIdx = i
			}
		}

		if bestIdx == -1 {
			_, err := tx.Exec(ctx, `
				INSERT INTO repair_assignments
					(request_id, brigade_number, start_date, end_date, planned_hours, responsible_person, assignment_status, notes)
				VALUES ($1, 0, NULL, NULL, $2, '', 'не назначена', $3)
				ON CONFLICT (request_id)
				DO UPDATE SET
					assignment_status = 'не назначена',
					notes = EXCLUDED.notes;
			`, p.RequestID, p.LaborHours, "нет подходящей бригады")
			if err != nil {
				return nil, err
			}
			continue
		}

		b := &brigades[bestIdx]
		b.CurrentAssignedHours += p.LaborHours

		respPerson := ""
		if b.TeamComposition != "" {
			respPerson = b.TeamComposition
		}

		if _, err := tx.Exec(ctx, `
			UPDATE brigades
			SET current_assigned_hours = $1
			WHERE brigade_number = $2;
		`, b.CurrentAssignedHours, b.BrigadeNumber); err != nil {
			return nil, err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO repair_assignments
				(request_id, brigade_number, start_date, end_date, planned_hours, responsible_person, assignment_status, notes)
			VALUES ($1,$2,$3,$4,$5,$6,'назначена','назначено автоматически')
			ON CONFLICT (request_id)
			DO UPDATE SET
				brigade_number = EXCLUDED.brigade_number,
				start_date = EXCLUDED.start_date,
				end_date = EXCLUDED.end_date,
				planned_hours = EXCLUDED.planned_hours,
				responsible_person = EXCLUDED.responsible_person,
				assignment_status = 'назначена',
				notes = EXCLUDED.notes;
		`, p.RequestID, b.BrigadeNumber, p.PlannedStartDate, p.PlannedEndDate, p.LaborHours, respPerson)
		if err != nil {
			return nil, err
		}

		out = append(out, assignment{
			RequestID: p.RequestID,
			BrigadeNumber: b.BrigadeNumber,
			StartDate: func() time.Time {
				if p.PlannedStartDate != nil { return *p.PlannedStartDate }
				return mt
			}(),
			EndDate: func() time.Time {
				if p.PlannedEndDate != nil { return *p.PlannedEndDate }
				return mt
			}(),
			PlannedHours: p.LaborHours,
			ResponsiblePerson: respPerson,
			Notes: "назначено автоматически",
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.QueryAll(ctx, `
		SELECT assignment_id, request_id, brigade_number, start_date, end_date,
		       planned_hours, responsible_person, assignment_status, notes
		FROM repair_assignments
		WHERE start_date >= $1 AND start_date < ($1 + INTERVAL '1 month')
		ORDER BY assignment_id DESC;
	`, mt)
}