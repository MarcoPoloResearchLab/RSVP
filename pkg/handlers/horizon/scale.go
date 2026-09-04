package horizon

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
)

type horizonScale string

const (
	horizonScaleDay     horizonScale = "day"
	horizonScaleWeek    horizonScale = "week"
	horizonScaleMonth   horizonScale = "month"
	horizonScaleYear    horizonScale = "year"
	horizonScaleDefault              = horizonScaleMonth

	horizonScaleCookieName = "rsvp_horizon_scale"
	horizonScaleCookieAge  = 365 * 24 * 60 * 60
)

var errMalformedHorizonScale = errors.New("malformed horizon scale")

func parseHorizonScale(value string) (horizonScale, error) {
	scale := horizonScale(value)
	switch scale {
	case horizonScaleDay, horizonScaleWeek, horizonScaleMonth, horizonScaleYear:
		return scale, nil
	default:
		return "", errMalformedHorizonScale
	}
}

func horizonScaleFromRequest(request *http.Request) (horizonScale, bool, error) {
	scaleValues, scalePresent := request.URL.Query()[config.HorizonScaleParam]
	if scalePresent {
		if len(scaleValues) != 1 || scaleValues[0] == "" {
			return "", false, errMalformedHorizonScale
		}
		scale, scaleError := parseHorizonScale(scaleValues[0])
		return scale, true, scaleError
	}
	cookie, cookieError := request.Cookie(horizonScaleCookieName)
	if errors.Is(cookieError, http.ErrNoCookie) {
		return horizonScaleDefault, false, nil
	}
	if cookieError != nil {
		return "", false, errMalformedHorizonScale
	}
	scale, scaleError := parseHorizonScale(cookie.Value)
	return scale, false, scaleError
}

func setHorizonScaleCookie(responseWriter http.ResponseWriter, scale horizonScale) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     horizonScaleCookieName,
		Value:    string(scale),
		Path:     config.WebRoot,
		MaxAge:   horizonScaleCookieAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func horizonScaleEnd(start time.Time, location *time.Location, scale horizonScale) time.Time {
	return shiftHorizonScaleStart(start, location, scale, 1)
}

func shiftHorizonScaleStart(start time.Time, location *time.Location, scale horizonScale, direction int) time.Time {
	localStart := start.In(location)
	switch scale {
	case horizonScaleDay:
		return localStart.AddDate(0, 0, direction)
	case horizonScaleWeek:
		return localStart.AddDate(0, 0, 7*direction)
	case horizonScaleMonth:
		return shiftCalendarMonths(localStart, direction)
	case horizonScaleYear:
		return shiftCalendarMonths(localStart, 12*direction)
	default:
		panic("invalid Horizon scale")
	}
}

func shiftCalendarMonths(start time.Time, months int) time.Time {
	lastTargetDay := time.Date(start.Year(), start.Month()+time.Month(months)+1, 0, 12, 0, 0, 0, start.Location())
	day := min(start.Day(), lastTargetDay.Day())
	return time.Date(lastTargetDay.Year(), lastTargetDay.Month(), day, start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), start.Location())
}

func horizonScaleURL(scale horizonScale, start time.Time) string {
	query := url.Values{}
	query.Set(config.HorizonScaleParam, string(scale))
	query.Set(config.HorizonStartParam, start.Format(time.RFC3339))
	return config.WebHorizon + "?" + query.Encode()
}
