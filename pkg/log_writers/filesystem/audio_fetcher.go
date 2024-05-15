package filesystem

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"nuance.xaas-logging.event-log-collector/pkg/httpclient"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/oauth"
)

const (
	httpTimeout = 60
)

type AudioFetcher struct {
	httpclient.Http

	authenticator *oauth.Authenticator
	afssURL       *url.URL
}

func NewAudioFetcher(authenticator *oauth.Authenticator, apiEndpoint string) *AudioFetcher {
	endpoint, err := url.Parse(apiEndpoint)
	if err != nil {
		log.Errorf("invalid endpoint for fetching audio from AFSS: %v", err)
		return nil
	}

	return &AudioFetcher{
		authenticator: authenticator,
		afssURL:       endpoint,
		Http: httpclient.Http{
			HttpClient: http.Client{
				Timeout: time.Duration(time.Duration(httpTimeout) * time.Second),
			},
		}}
}

// header() creates a default HTTP header with bearer token
func (a *AudioFetcher) header(token oauth.Token) map[string][]string {
	return map[string][]string{
		"Authorization": {fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)},
		"Content-Type":  {"application/json"},
	}
}

// newAudioIntBuffer() converts a pcm byte buffer to an int buffer so it can be properly encoded to wav
func (a *AudioFetcher) newAudioIntBuffer(r io.Reader) (*audio.IntBuffer, error) {
	buf := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  16000,
		},
	}
	for {
		var sample int16
		err := binary.Read(r, binary.LittleEndian, &sample)
		switch {
		case err == io.EOF:
			return &buf, nil
		case err != nil:
			return nil, err
		}
		buf.Data = append(buf.Data, int(sample))
	}
}

// saveAsWav() takes a pcm byte array and saves it as a wav file to the destination file
func (a *AudioFetcher) saveAsWav(pcm []byte, dst string, rate int) error {
	wavfile, err := os.Create(filepath.Clean(dst))
	if err != nil {
		return err
	}
	defer wavfile.Close()

	encoder := wav.NewEncoder(wavfile,
		rate,
		16,
		1,
		1)

	buf, err := a.newAudioIntBuffer(bytes.NewReader(pcm))
	if err != nil {
		return err
	}

	if err = encoder.Write(buf); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}

	return nil
}

// saveAudioToFile() saves audio returned from AFSS to file
func (a *AudioFetcher) saveAudioToFile(audio []byte, dst string, rate int) error {

	// Make sure destination directory exists
	err := os.MkdirAll(filepath.Dir(dst), 0750)
	if err != nil {
		return fmt.Errorf("couldn't create audio file - %v", err)
	}

	// Encode the pcm to wav to make it easy the audio easy to listen to
	return a.saveAsWav(audio, dst, rate)
}

// fetchAudio() calls the AFSS endpoint and downloads the target audio by urn
func (a *AudioFetcher) fetchAudio(token *oauth.Token, urn string) ([]byte, error) {

	// build AFSS request uri
	v := url.Values{}
	v.Set("urn", urn)
	uri, _ := url.ParseRequestURI(fmt.Sprintf("%v?%v", a.afssURL, v.Encode()))
	log.Debugf("afss endpoint: %v", uri)

	// Call AFSS to fetch audio
	statusCode, response, err := a.Http.Get(uri, a.header(*token))
	if err != nil {
		log.Errorf("%v", err)
		return nil, err
	}

	// Process status code
	switch {
	case statusCode == 200:
		return response, nil
	case statusCode == 404:
		return nil, fmt.Errorf("audio urn not found: %v", urn)
	default:
		return nil, errors.New(string(response))
	}
}

// Download() downloads audio from AFSS and saves it to file
func (a *AudioFetcher) Download(urn, dst string, rate int) error {
	token, err := a.authenticator.Authenticate()
	if err != nil {
		return err
	}

	pcm, err := a.fetchAudio(token, urn)
	if err != nil {
		return err
	}

	return a.saveAudioToFile(pcm, dst, rate)
}
