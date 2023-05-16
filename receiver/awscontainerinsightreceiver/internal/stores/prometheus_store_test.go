package stores

import (
	"context"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestPrometheusStore(t *testing.T) {
	storage := map[string]float64{}
	store := NewPrometheusStore(zap.NewNop(), &storage)
	ctx := context.TODO()

	myAppender := store.Appender(ctx)
	assert.NotNil(t, myAppender)

	response, err := myAppender.Append(0, labels.FromStrings(model.MetricNameLabel, "myLabel"), 0, 1234.5)
	assert.NoError(t, err)
	assert.Zero(t, response)

	assert.Equal(t, 1234.5, storage["{__name__=\"myLabel\"}"])
}

func TestAppendNoMetricName(t *testing.T) {
	storage := map[string]float64{}
	store := NewPrometheusStore(zap.NewNop(), &storage)
	ctx := context.TODO()

	myAppender := store.Appender(ctx)
	assert.NotNil(t, myAppender)

	response, err := myAppender.Append(0, labels.FromStrings("not_metric_name", "myLabel"), 0, 1234.5)
	assert.Error(t, err)
	assert.Zero(t, response)
}

func TestNoOpMethods(t *testing.T) {
	storage := map[string]float64{}
	store := NewPrometheusStore(zap.NewNop(), &storage)
	ctx := context.TODO()

	myAppender := store.Appender(ctx)
	assert.NotNil(t, myAppender)

	err := myAppender.Commit()
	assert.NoError(t, err)

	err = myAppender.Rollback()
	assert.NoError(t, err)

	myLabel := labels.FromStrings(model.MetricNameLabel, "myLabel")
	myExemplar := exemplar.Exemplar{}
	response, err := myAppender.AppendExemplar(0, myLabel, myExemplar)
	assert.NoError(t, err)
	assert.Zero(t, response)
	assert.Zero(t, len(storage))

	myHistogram := histogram.Histogram{}
	myFloatHistogram := histogram.FloatHistogram{}
	response, err = myAppender.AppendHistogram(0, myLabel, 0, &myHistogram, &myFloatHistogram)
	assert.NoError(t, err)
	assert.Zero(t, response)
	assert.Zero(t, len(storage))

	myMetadata := metadata.Metadata{}
	response, err = myAppender.UpdateMetadata(0, myLabel, myMetadata)
	assert.NoError(t, err)
	assert.Zero(t, response)
	assert.Zero(t, len(storage))
}
