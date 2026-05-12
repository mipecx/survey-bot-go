// Package sheets provides a Google Sheets client for appending survey results
// and initialising the spreadsheet structure at application startup.
package sheets

import (
	"context"
	"log/slog"
	"os"

	"github.com/mipecx/survey-bot-go/internal/ctxlog"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Client wraps the Google Sheets API service and provides high-level methods
// for managing survey result sheets.
type Client struct {
	service       *sheets.Service
	spreadsheetID string
	logger        *slog.Logger
}

// SheetConfig describes a single worksheet: its display name and the ordered
// list of column headers written to row 1 on first initialisation.
type SheetConfig struct {
	Name    string
	Headers []string
}

// New creates an authenticated Google Sheets client using a Service Account
// key file. The credentials file is read from disk and authenticated via OAuth2.
// spreadsheetID is the ID from the Google Sheets URL:
// https://docs.google.com/spreadsheets/d/<spreadsheetID>/edit
func New(
	ctx context.Context,
	credentialsFile string,
	spreadsheetID string,
	logger *slog.Logger,
) (*Client, error) {

	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, err
	}

	creds, err := google.CredentialsFromJSONWithType(
		ctx,
		data,
		google.ServiceAccount,
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		return nil, err
	}

	srv, err := sheets.NewService(
		ctx,
		option.WithTokenSource(creds.TokenSource),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		service:       srv,
		spreadsheetID: spreadsheetID,
		logger:        logger,
	}, nil
}

// AppendRow inserts a new row of values at the end of the named sheet.
// values must be ordered to match the headers defined in SheetConfig.
// Uses INSERT_ROWS mode - never overwrites existing data.
func (c *Client) AppendRow(ctx context.Context, sheetName string, values []any) error {
	row := &sheets.ValueRange{
		Values: [][]any{values},
	}
	_, err := c.service.Spreadsheets.Values.
		Append(c.spreadsheetID, sheetName+"!A1", row).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	if err != nil {
		c.logger.Error("failed to append row to sheet", "sheet", sheetName, "error", err)
		return err
	}
	return nil
}

// InitSheets ensures all configured worksheets exist and have headers in row 1.
// Called once at application startup. For each SheetConfig:
//  1. Creates the worksheet if it does not already exist.
//  2. Writes headers to A1 if the row is empty.
//
// Existing data is never modified or deleted.
func (c *Client) InitSheets(ctx context.Context, configs []SheetConfig) error {
	logger := ctxlog.LoggerFromCtx(ctx, c.logger)

	spreadsheet, err := c.service.Spreadsheets.Get(c.spreadsheetID).Context(ctx).Do()
	if err != nil {
		logger.Error("failed to get spreadsheet", "error", err)
	}
	existing := make(map[string]bool)
	for _, sheet := range spreadsheet.Sheets {
		existing[sheet.Properties.Title] = true
	}
	var requests []*sheets.Request
	for _, cfg := range configs {
		if !existing[cfg.Name] {
			requests = append(requests, &sheets.Request{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{
						Title: cfg.Name,
					},
				},
			})
		}
	}
	if len(requests) > 0 {
		_, err = c.service.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
			Requests: requests,
		}).Context(ctx).Do()
		if err != nil {
			logger.Error("failed to create sheets", "error", err)
		}
	}
	for _, cfg := range configs {
		rangeStr := cfg.Name + "!A1"
		resp, err := c.service.Spreadsheets.Values.Get(c.spreadsheetID, rangeStr).Context(ctx).Do()
		if err != nil || len(resp.Values) == 0 {
			headers := make([]any, len(cfg.Headers))
			for i, h := range cfg.Headers {
				headers[i] = h
			}
			_, err = c.service.Spreadsheets.Values.
				Update(c.spreadsheetID, rangeStr, &sheets.ValueRange{
					Values: [][]any{headers},
				}).
				ValueInputOption("RAW").
				Context(ctx).
				Do()
			if err != nil {
				c.logger.Error("failed to write headers", "sheet", cfg.Name, "error", err)
			}
		}
	}
	c.logger.Info("sheets initialized successfully")
	return nil
}
