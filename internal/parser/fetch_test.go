package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPostsListJSON = `{"success":true,"data":{"content":[
{"id":4757,"title":"Оновлено графік погодинних відключень (ГПВ) на 6 жовтня","slug":"onovleno-hrafik-na-6-zhovtnya","publishedAt":"2026-10-06T16:42:00"},
{"id":4655,"title":"Графік погодинних відключень (ГПВ) на 6 жовтня","slug":"hrafik-na-6-zhovtnya","publishedAt":"2026-10-06T12:23:00"},
{"id":4738,"title":"З Днем захисників і захисниць України!","slug":"z-dnem-zakhysnykiv","publishedAt":"2026-10-01T08:26:00"}
],"page":0,"size":20,"totalPages":1}}`

const testPost4757JSON = `{"success":true,"data":{"id":4757,"title":"Оновлено графік погодинних відключень (ГПВ) на 6 жовтня","content":"<p>Години відсутності електропостачання:</p><p>1.1 15:00 - 17:00, 22:00 - 24:00</p><p>6.2 20:00 - 22:00</p>","publishedAt":"2026-10-06T16:42:00"}}`

const testPost4655JSON = `{"success":true,"data":{"id":4655,"title":"Графік погодинних відключень (ГПВ) на 6 жовтня","content":"<p>1.1 15:00 - 17:00</p>","publishedAt":"2026-10-06T12:23:00"}}`

const test404HTML = `<html><head><title>404 Not Found</title></head><body></body></html>`

func newTestPostsServer(t *testing.T, details map[string]string) (*httptest.Server, *[]string) {
	t.Helper()

	var mu sync.Mutex
	var requested []string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/posts/category/news", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testPostsListJSON))
	})
	mux.HandleFunc("/api/v1/posts/", func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Path[len("/api/v1/posts/"):]
		mu.Lock()
		requested = append(requested, slug)
		mu.Unlock()

		body, ok := details[slug]
		if !ok {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(test404HTML))
			return
		}
		assert.Equal(t, "uk", r.URL.Query().Get("lang"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &requested
}

func TestFetchScheduleNews_ParsesListAndDetails(t *testing.T) {
	server, requested := newTestPostsServer(t, map[string]string{
		"onovleno-hrafik-na-6-zhovtnya": testPost4757JSON,
		"hrafik-na-6-zhovtnya":          testPost4655JSON,
	})

	news, err := fetchScheduleNews(context.Background(), server.Client(), server.URL+"/api/v1/posts/category/news?lang=uk&page=0&size=20")

	require.NoError(t, err)
	require.Len(t, news, 2)
	assert.ElementsMatch(t, []string{"onovleno-hrafik-na-6-zhovtnya", "hrafik-na-6-zhovtnya"}, *requested,
		"details must be fetched only for schedule posts")

	assert.Equal(t, 4757, news[0].ID)
	assert.Equal(t, "Оновлено графік погодинних відключень (ГПВ) на 6 жовтня", news[0].Title)
	assert.True(t, time.Date(2026, time.October, 6, 16, 42, 0, 0, kievLocation).Equal(news[0].Date))
	assert.Contains(t, news[0].HtmlBody, "<p>1.1 15:00 - 17:00, 22:00 - 24:00</p>")

	sch, ok := buildScheduleFromNews(news[0])
	require.True(t, ok)
	assert.Equal(t, 4757, sch.NewsID)
	assert.Equal(t, "15:00 - 17:00, 22:00 - 24:00", sch.Col1_1)
	assert.Equal(t, "20:00 - 22:00", sch.Col6_2)
	assert.Regexp(t, `^\d{4}-10-06$`, sch.ScheduleDate)
}

func TestFetchScheduleNews_SkipsFailedDetail(t *testing.T) {
	server, _ := newTestPostsServer(t, map[string]string{
		"hrafik-na-6-zhovtnya": testPost4655JSON,
	})

	news, err := fetchScheduleNews(context.Background(), server.Client(), server.URL+"/api/v1/posts/category/news?lang=uk")

	require.NoError(t, err)
	require.Len(t, news, 1)
	assert.Equal(t, 4655, news[0].ID)
}

func TestFetchScheduleNews_ListErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(test404HTML))
	}))
	t.Cleanup(server.Close)

	_, err := fetchScheduleNews(context.Background(), server.Client(), server.URL+"/api/v1/posts/category/news")

	assert.Error(t, err)
}

func TestBuildPostDetailURL(t *testing.T) {
	tests := []struct {
		name    string
		listURL string
		slug    string
		want    string
		wantErr bool
	}{
		{
			name:    "production list URL",
			listURL: "https://www.cherkasyoblenergo.com/api/v1/posts/category/news?lang=uk&page=0&size=20",
			slug:    "hrafik-na-6-zhovtnya",
			want:    "https://www.cherkasyoblenergo.com/api/v1/posts/hrafik-na-6-zhovtnya?lang=uk",
		},
		{
			name:    "defaults lang to uk",
			listURL: "https://www.cherkasyoblenergo.com/api/v1/posts/category/news",
			slug:    "abc",
			want:    "https://www.cherkasyoblenergo.com/api/v1/posts/abc?lang=uk",
		},
		{
			name:    "keeps lang from list URL",
			listURL: "https://www.cherkasyoblenergo.com/api/v1/posts/category/news?lang=en",
			slug:    "abc",
			want:    "https://www.cherkasyoblenergo.com/api/v1/posts/abc?lang=en",
		},
		{
			name:    "legacy URL is rejected",
			listURL: "https://gita.cherkasyoblenergo.com/obl-main-controller/api/news2?size=18&category=1&page=0",
			slug:    "abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPostDetailURL(tt.listURL, tt.slug)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
