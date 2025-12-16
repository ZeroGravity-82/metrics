package repository

import (
	"context"
	"database/sql"
	"testing"
	"zerogravity-82/metrics/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDbStorage_CanInstantiate(t *testing.T) {
	// Arrange
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")

	// Act
	ds := NewDbStorage(sqlxDb)

	// Assert
	assert.Equal(t, sqlxDb, ds.db)
}

func TestUpdateMetricInDbStorage_FailWithEmptyName(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")
	ds := NewDbStorage(sqlxDb)
	m := model.Metrics{ID: "", MType: model.Counter, Delta: int64Pointer(777), Value: nil}

	// Act
	err = ds.UpdateMetric(ctx, m)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMetricNotFound)
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestUpdateMetricInDbStorage_CanAddNewMetric(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")

	t.Run("can add new counter", func(t *testing.T) {
		// Arrange
		ds := NewDbStorage(sqlxDb)
		m := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(777), Value: nil}
		delta := sql.NullInt64{Int64: 777, Valid: true}
		value := sql.NullFloat64{Float64: 0, Valid: false}
		mock.ExpectExec("INSERT INTO metric").
			WithArgs("PollCount", model.Counter, delta, value).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Act
		err = ds.UpdateMetric(ctx, m)

		// Assert
		require.NoError(t, err)
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})

	t.Run("can add new gauge", func(t *testing.T) {
		// Arrange
		ds := NewDbStorage(sqlxDb)
		m := model.Metrics{ID: "RandomValue", MType: model.Gauge, Delta: nil, Value: float64Pointer(123.45)}
		delta := sql.NullInt64{Int64: 0, Valid: false}
		value := sql.NullFloat64{Float64: 123.45, Valid: true}
		mock.ExpectExec("INSERT INTO metric").
			WithArgs("RandomValue", model.Gauge, delta, value).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Act
		err = ds.UpdateMetric(ctx, m)

		// Assert
		require.NoError(t, err)
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})
}

func TestUpdateMetricsInDbStorage_FailEvenWithOneSingleInvalidMetric(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")
	ds := NewDbStorage(sqlxDb)
	mu := []model.Metrics{
		{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(111), Value: nil},
		{ID: "RandomValue", MType: "unsupported", Delta: nil, Value: float64Pointer(234.56)},
	}
	mock.ExpectBegin()
	mock.ExpectRollback()

	// Act
	err = ds.UpdateMetrics(ctx, mu)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedMetricType)
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestUpdateMetricsInDbStorage_CanAddNewMetrics(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")
	ds := NewDbStorage(sqlxDb)
	mu := []model.Metrics{
		{ID: "PollCount", MType: model.Counter, Delta: int64Pointer(111), Value: nil},
		{ID: "RandomValue", MType: model.Gauge, Delta: nil, Value: float64Pointer(234.56)},
		{ID: "Alloc", MType: model.Gauge, Delta: nil, Value: float64Pointer(3329920)},
		{ID: "BuckHashSys", MType: model.Gauge, Delta: nil, Value: float64Pointer(1443559)},
		{ID: "Frees", MType: model.Gauge, Delta: nil, Value: float64Pointer(84)},
		{ID: "GCCPUFraction", MType: model.Gauge, Delta: nil, Value: float64Pointer(0)},
		{ID: "GCSys", MType: model.Gauge, Delta: nil, Value: float64Pointer(1888528)},
		{ID: "HeapAlloc", MType: model.Gauge, Delta: nil, Value: float64Pointer(3329920)},
		{ID: "HeapIdle", MType: model.Gauge, Delta: nil, Value: float64Pointer(3948544)},
		{ID: "HeapInuse", MType: model.Gauge, Delta: nil, Value: float64Pointer(4112384)},
		{ID: "HeapObjects", MType: model.Gauge, Delta: nil, Value: float64Pointer(1465)},
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO metric").
		WithArgs(
			"PollCount", "counter", sql.NullInt64{Int64: 111, Valid: true}, sql.NullFloat64{Float64: 0, Valid: false},
			"RandomValue", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 234.56, Valid: true},
			"Alloc", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 3329920, Valid: true},
			"BuckHashSys", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 1443559, Valid: true},
			"Frees", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 84, Valid: true},
			"GCCPUFraction", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 0, Valid: true},
			"GCSys", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 1888528, Valid: true},
			"HeapAlloc", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 3329920, Valid: true},
			"HeapIdle", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 3948544, Valid: true},
			"HeapInuse", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 4112384, Valid: true},
		).
		WillReturnResult(sqlmock.NewResult(1, 10))
	mock.ExpectExec("INSERT INTO metric").
		WithArgs(
			"HeapObjects", "gauge", sql.NullInt64{Int64: 0, Valid: false}, sql.NullFloat64{Float64: 1465, Valid: true},
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Act
	err = ds.UpdateMetrics(ctx, mu)

	// Assert
	require.NoError(t, err)
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestGetMetricInDbStorage(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")
	ds := NewDbStorage(sqlxDb)

	t.Run("fail when not found by name", func(t *testing.T) {
		// Arrange
		mock.ExpectQuery("SELECT id, type, delta, value FROM metric").
			WithArgs("unknown", "counter").
			WillReturnError(sql.ErrNoRows)

		// Act
		_, err = ds.GetMetric(ctx, model.Counter, "unknown")

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrMetricNotFound)
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})

	t.Run("can get counter by name", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "type", "delta", "value"}).
			AddRow(
				"PollCount",
				"counter",
				sql.NullInt64{Int64: 777, Valid: true},
				sql.NullFloat64{Float64: 0, Valid: false},
			)
		mock.ExpectQuery("SELECT id, type, delta, value FROM metric").
			WithArgs("PollCount", "counter").
			WillReturnRows(rows)

		// Act
		m, err := ds.GetMetric(ctx, model.Counter, "PollCount")

		// Assert
		require.NoError(t, err)
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
		assert.Equal(t, "PollCount", m.ID)
		assert.Equal(t, "counter", m.MType)
		assert.Equal(t, int64(777), *m.Delta)
		assert.Nil(t, m.Value)
	})
	t.Run("can get gauge by name", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "type", "delta", "value"}).
			AddRow(
				"RandomValue",
				"gauge",
				sql.NullInt64{Int64: 0, Valid: false},
				sql.NullFloat64{Float64: 123.45, Valid: true},
			)
		mock.ExpectQuery("SELECT id, type, delta, value FROM metric").
			WithArgs("RandomValue", "gauge").
			WillReturnRows(rows)

		// Act
		m, err := ds.GetMetric(ctx, model.Gauge, "RandomValue")

		// Assert
		require.NoError(t, err)
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
		assert.Equal(t, "RandomValue", m.ID)
		assert.Equal(t, "gauge", m.MType)
		assert.Nil(t, m.Delta)
		assert.Equal(t, float64(123.45), *m.Value)
	})
}

func TestGetAllInDbStorage(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")
	ds := NewDbStorage(sqlxDb)

	rows := sqlmock.NewRows([]string{"id", "type", "delta", "value"}).
		AddRow(
			"PollCount",
			"counter",
			sql.NullInt64{Int64: 777, Valid: true},
			sql.NullFloat64{Float64: 0, Valid: false},
		).
		AddRow(
			"RandomValue",
			"gauge",
			sql.NullInt64{Int64: 0, Valid: false},
			sql.NullFloat64{Float64: 123.45, Valid: true},
		)
	mock.ExpectQuery("SELECT id, type, delta, value FROM metric").
		WillReturnRows(rows)

	// Act
	metrics, err := ds.GetAll(ctx)

	// Assert
	require.NoError(t, err)
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
	assert.Equal(
		t,
		metrics["PollCount"],
		model.Metrics{ID: "PollCount", MType: "counter", Delta: int64Pointer(777), Value: nil},
	)
	assert.Equal(
		t,
		metrics["RandomValue"],
		model.Metrics{ID: "RandomValue", MType: "gauge", Delta: nil, Value: float64Pointer(123.45)},
	)
	assert.Len(t, metrics, 2)
}

func TestPingInDbStorage(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")
	ds := NewDbStorage(sqlxDb)

	mock.ExpectPing()

	// Act
	err = ds.Ping(ctx)

	// Assert
	require.NoError(t, err)
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestCloseInDbStorage(t *testing.T) {
	// Arrange
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDb := sqlx.NewDb(db, "sqlmock")
	ds := NewDbStorage(sqlxDb)

	mock.ExpectClose()

	// Act
	err = ds.Close()

	// Assert
	require.NoError(t, err)
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}
