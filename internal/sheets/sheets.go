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

type Client struct {
	service       *sheets.Service
	spreadsheetID string
	logger        *slog.Logger
}

type SheetConfig struct {
	Name    string
	Headers []string
}

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
