package connectors

// PresetDefs returns all built-in connector definitions.
func PresetDefs() []*ConnectorDef {
	return []*ConnectorDef{
		githubDef(),
		gmailDef(),
		googleCalendarDef(),
		outlookMailDef(),
		outlookCalendarDef(),
		notionDef(),
		slackDef(),
		linearDef(),
		jiraDef(),
	}
}

func githubDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "github",
		Name:        "GitHub",
		Description: "Access repositories, issues, pull requests, and code on GitHub.",
		Icon:        "github",
		Category:    "developer",
		AuthType:    "token",
		Actions: []ActionDef{
			{ID: "list_repos", Name: "List Repositories", Description: "List your GitHub repositories.", Parameters: []ParamDef{
				{Name: "visibility", Type: "string", Description: "all, public, or private"},
				{Name: "sort", Type: "string", Description: "created, updated, pushed, full_name"},
			}},
			{ID: "get_repo", Name: "Get Repository", Description: "Get details of a repository.", Parameters: []ParamDef{
				{Name: "owner", Type: "string", Description: "Repository owner", Required: true},
				{Name: "repo", Type: "string", Description: "Repository name", Required: true},
			}},
			{ID: "list_issues", Name: "List Issues", Description: "List issues in a repository.", Parameters: []ParamDef{
				{Name: "owner", Type: "string", Description: "Repository owner", Required: true},
				{Name: "repo", Type: "string", Description: "Repository name", Required: true},
				{Name: "state", Type: "string", Description: "open, closed, or all"},
			}},
			{ID: "create_issue", Name: "Create Issue", Description: "Create a new issue.", Parameters: []ParamDef{
				{Name: "owner", Type: "string", Description: "Repository owner", Required: true},
				{Name: "repo", Type: "string", Description: "Repository name", Required: true},
				{Name: "title", Type: "string", Description: "Issue title", Required: true},
				{Name: "body", Type: "string", Description: "Issue body"},
			}},
			{ID: "list_prs", Name: "List Pull Requests", Description: "List pull requests in a repository.", Parameters: []ParamDef{
				{Name: "owner", Type: "string", Description: "Repository owner", Required: true},
				{Name: "repo", Type: "string", Description: "Repository name", Required: true},
				{Name: "state", Type: "string", Description: "open, closed, or all"},
			}},
			{ID: "search_code", Name: "Search Code", Description: "Search for code on GitHub.", Parameters: []ParamDef{
				{Name: "query", Type: "string", Description: "Search query", Required: true},
			}},
			{ID: "get_file", Name: "Get File Contents", Description: "Get contents of a file in a repository.", Parameters: []ParamDef{
				{Name: "owner", Type: "string", Description: "Repository owner", Required: true},
				{Name: "repo", Type: "string", Description: "Repository name", Required: true},
				{Name: "path", Type: "string", Description: "File path", Required: true},
				{Name: "ref", Type: "string", Description: "Branch or commit SHA"},
			}},
		},
	}
}

func gmailDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "gmail",
		Name:        "Gmail",
		Description: "Read, send, and manage your Gmail messages.",
		Icon:        "mail",
		Category:    "communication",
		AuthType:    "oauth2",
		Beta:        true,
		Scopes:      []string{"https://www.googleapis.com/auth/gmail.modify"},
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		RevokeURL:   "https://oauth2.googleapis.com/revoke",
		Actions: []ActionDef{
			{ID: "list_messages", Name: "List Messages", Description: "List recent email messages.", Parameters: []ParamDef{
				{Name: "query", Type: "string", Description: "Search query (Gmail search syntax)"},
				{Name: "max_results", Type: "number", Description: "Maximum number of results (default 10)"},
			}},
			{ID: "get_message", Name: "Get Message", Description: "Get a specific email message.", Parameters: []ParamDef{
				{Name: "message_id", Type: "string", Description: "Message ID", Required: true},
			}},
			{ID: "send_message", Name: "Send Message", Description: "Send a new email.", Parameters: []ParamDef{
				{Name: "to", Type: "string", Description: "Recipient email", Required: true},
				{Name: "subject", Type: "string", Description: "Email subject", Required: true},
				{Name: "body", Type: "string", Description: "Email body", Required: true},
			}},
			{ID: "list_labels", Name: "List Labels", Description: "List Gmail labels."},
		},
	}
}

func googleCalendarDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "google_calendar",
		Name:        "Google Calendar",
		Description: "View and manage your Google Calendar events.",
		Icon:        "calendar",
		Category:    "productivity",
		AuthType:    "oauth2",
		Scopes:      []string{"https://www.googleapis.com/auth/calendar"},
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		Actions: []ActionDef{
			{ID: "list_events", Name: "List Events", Description: "List upcoming events.", Parameters: []ParamDef{
				{Name: "time_min", Type: "string", Description: "Start time (RFC3339)"},
				{Name: "time_max", Type: "string", Description: "End time (RFC3339)"},
				{Name: "max_results", Type: "number", Description: "Maximum results"},
			}},
			{ID: "create_event", Name: "Create Event", Description: "Create a new calendar event.", Parameters: []ParamDef{
				{Name: "summary", Type: "string", Description: "Event title", Required: true},
				{Name: "start", Type: "string", Description: "Start time (RFC3339)", Required: true},
				{Name: "end", Type: "string", Description: "End time (RFC3339)", Required: true},
				{Name: "description", Type: "string", Description: "Event description"},
			}},
			{ID: "delete_event", Name: "Delete Event", Description: "Delete a calendar event.", Parameters: []ParamDef{
				{Name: "event_id", Type: "string", Description: "Event ID", Required: true},
			}},
		},
	}
}

func outlookMailDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "outlook_mail",
		Name:        "Outlook Mail",
		Description: "Read, send, and manage your Outlook emails.",
		Icon:        "mail",
		Category:    "communication",
		AuthType:    "oauth2",
		Beta:        true,
		Scopes:      []string{"Mail.ReadWrite", "Mail.Send"},
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Actions: []ActionDef{
			{ID: "list_messages", Name: "List Messages", Description: "List recent emails."},
			{ID: "get_message", Name: "Get Message", Description: "Get a specific email.", Parameters: []ParamDef{
				{Name: "message_id", Type: "string", Description: "Message ID", Required: true},
			}},
			{ID: "send_message", Name: "Send Message", Description: "Send a new email.", Parameters: []ParamDef{
				{Name: "to", Type: "string", Description: "Recipient", Required: true},
				{Name: "subject", Type: "string", Description: "Subject", Required: true},
				{Name: "body", Type: "string", Description: "Body", Required: true},
			}},
		},
	}
}

func outlookCalendarDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "outlook_calendar",
		Name:        "Outlook Calendar",
		Description: "View and manage your Outlook Calendar events.",
		Icon:        "calendar",
		Category:    "productivity",
		AuthType:    "oauth2",
		Beta:        true,
		Scopes:      []string{"Calendars.ReadWrite"},
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Actions: []ActionDef{
			{ID: "list_events", Name: "List Events", Description: "List upcoming events."},
			{ID: "create_event", Name: "Create Event", Description: "Create a calendar event.", Parameters: []ParamDef{
				{Name: "subject", Type: "string", Description: "Event title", Required: true},
				{Name: "start", Type: "string", Description: "Start time (ISO 8601)", Required: true},
				{Name: "end", Type: "string", Description: "End time (ISO 8601)", Required: true},
			}},
		},
	}
}

func notionDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "notion",
		Name:        "Notion",
		Description: "Search, read, and create pages in your Notion workspace.",
		Icon:        "notion",
		Category:    "productivity",
		AuthType:    "token",
		Actions: []ActionDef{
			{ID: "search", Name: "Search", Description: "Search pages and databases.", Parameters: []ParamDef{
				{Name: "query", Type: "string", Description: "Search query", Required: true},
			}},
			{ID: "get_page", Name: "Get Page", Description: "Get a Notion page.", Parameters: []ParamDef{
				{Name: "page_id", Type: "string", Description: "Page ID", Required: true},
			}},
			{ID: "create_page", Name: "Create Page", Description: "Create a new page.", Parameters: []ParamDef{
				{Name: "parent_id", Type: "string", Description: "Parent page or database ID", Required: true},
				{Name: "title", Type: "string", Description: "Page title", Required: true},
				{Name: "content", Type: "string", Description: "Page content (markdown)"},
			}},
		},
	}
}

func slackDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "slack",
		Name:        "Slack",
		Description: "Send messages and manage channels in your Slack workspace.",
		Icon:        "slack",
		Category:    "communication",
		AuthType:    "token",
		Actions: []ActionDef{
			{ID: "send_message", Name: "Send Message", Description: "Send a message to a Slack channel.", Parameters: []ParamDef{
				{Name: "channel", Type: "string", Description: "Channel name or ID", Required: true},
				{Name: "text", Type: "string", Description: "Message text", Required: true},
			}},
			{ID: "list_channels", Name: "List Channels", Description: "List Slack channels."},
		},
	}
}

func linearDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "linear",
		Name:        "Linear",
		Description: "Manage issues and projects in Linear.",
		Icon:        "linear",
		Category:    "developer",
		AuthType:    "token",
		Beta:        true,
		Actions: []ActionDef{
			{ID: "list_issues", Name: "List Issues", Description: "List issues.", Parameters: []ParamDef{
				{Name: "state", Type: "string", Description: "Filter by state"},
			}},
			{ID: "create_issue", Name: "Create Issue", Description: "Create a new issue.", Parameters: []ParamDef{
				{Name: "title", Type: "string", Description: "Issue title", Required: true},
				{Name: "description", Type: "string", Description: "Issue description"},
				{Name: "team_id", Type: "string", Description: "Team ID", Required: true},
			}},
		},
	}
}

func jiraDef() *ConnectorDef {
	return &ConnectorDef{
		ID:          "jira",
		Name:        "Jira",
		Description: "Manage issues and projects in Atlassian Jira.",
		Icon:        "jira",
		Category:    "developer",
		AuthType:    "token",
		Beta:        true,
		Actions: []ActionDef{
			{ID: "search_issues", Name: "Search Issues", Description: "Search issues using JQL.", Parameters: []ParamDef{
				{Name: "jql", Type: "string", Description: "JQL query", Required: true},
			}},
			{ID: "create_issue", Name: "Create Issue", Description: "Create a new issue.", Parameters: []ParamDef{
				{Name: "project", Type: "string", Description: "Project key", Required: true},
				{Name: "summary", Type: "string", Description: "Issue summary", Required: true},
				{Name: "issue_type", Type: "string", Description: "Issue type (Bug, Task, Story)", Required: true},
				{Name: "description", Type: "string", Description: "Issue description"},
			}},
		},
	}
}
