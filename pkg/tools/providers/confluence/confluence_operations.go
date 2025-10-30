package confluence

import (
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// GetOperationMappings returns Confluence-specific operation mappings
func (p *ConfluenceProvider) GetOperationMappings() map[string]providers.OperationMapping {
	return map[string]providers.OperationMapping{
		// Content operations (Pages and Blog Posts)
		"content/list": {
			OperationID:    "getContent",
			Method:         "GET",
			PathTemplate:   "/content",
			RequiredParams: []string{},
			OptionalParams: []string{"spaceKey", "type", "status", "expand", "limit", "start"},
		},
		"content/get": {
			OperationID:    "getContentById",
			Method:         "GET",
			PathTemplate:   "/content/{id}",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"status", "version", "expand"},
		},
		"content/create": {
			OperationID:    "createContent",
			Method:         "POST",
			PathTemplate:   "/content",
			RequiredParams: []string{"type", "title", "space"},
			OptionalParams: []string{"body", "status", "ancestors"},
		},
		"content/update": {
			OperationID:    "updateContent",
			Method:         "PUT",
			PathTemplate:   "/content/{id}",
			RequiredParams: []string{"id", "version", "title", "type"},
			OptionalParams: []string{"body", "status"},
		},
		"content/delete": {
			OperationID:    "deleteContent",
			Method:         "DELETE",
			PathTemplate:   "/content/{id}",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"status"},
		},
		"content/search": {
			OperationID:    "searchContent",
			Method:         "GET",
			PathTemplate:   "/search",
			RequiredParams: []string{"cql"},
			OptionalParams: []string{"cqlcontext", "expand", "cursor", "limit"},
		},
		"content/children": {
			OperationID:    "getContentChildren",
			Method:         "GET",
			PathTemplate:   "/content/{id}/child",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"expand", "parentVersion", "limit", "start"},
		},
		"content/descendants": {
			OperationID:    "getContentDescendants",
			Method:         "GET",
			PathTemplate:   "/content/{id}/descendant",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"expand", "limit", "start"},
		},
		"content/versions": {
			OperationID:    "getContentVersions",
			Method:         "GET",
			PathTemplate:   "/content/{id}/version",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"expand", "limit", "start"},
		},
		"content/restore": {
			OperationID:    "restoreContentVersion",
			Method:         "POST",
			PathTemplate:   "/content/{id}/version",
			RequiredParams: []string{"id", "versionNumber"},
			OptionalParams: []string{"message"},
		},

		// Space operations
		"space/list": {
			OperationID:    "getSpaces",
			Method:         "GET",
			PathTemplate:   "/space",
			RequiredParams: []string{},
			OptionalParams: []string{"spaceKey", "type", "status", "expand", "limit", "start"},
		},
		"space/get": {
			OperationID:    "getSpace",
			Method:         "GET",
			PathTemplate:   "/space/{spaceKey}",
			RequiredParams: []string{"spaceKey"},
			OptionalParams: []string{"expand"},
		},
		"space/content": {
			OperationID:    "getSpaceContent",
			Method:         "GET",
			PathTemplate:   "/space/{spaceKey}/content",
			RequiredParams: []string{"spaceKey"},
			OptionalParams: []string{"type", "depth", "expand", "limit", "start"},
		},
		"space/permissions": {
			OperationID:    "getSpacePermissions",
			Method:         "GET",
			PathTemplate:   "/space/{spaceKey}/permission",
			RequiredParams: []string{"spaceKey"},
			OptionalParams: []string{"expand", "limit", "start"},
		},

		// Attachment operations
		"attachment/list": {
			OperationID:    "getAttachments",
			Method:         "GET",
			PathTemplate:   "/content/{id}/child/attachment",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"expand", "limit", "start", "filename", "mediaType"},
		},
		"attachment/get": {
			OperationID:    "getAttachment",
			Method:         "GET",
			PathTemplate:   "/content/{id}/child/attachment/{attachmentId}",
			RequiredParams: []string{"id", "attachmentId"},
			OptionalParams: []string{"expand", "version"},
		},
		"attachment/create": {
			OperationID:    "createAttachment",
			Method:         "POST",
			PathTemplate:   "/content/{id}/child/attachment",
			RequiredParams: []string{"id", "file"},
			OptionalParams: []string{"comment", "minorEdit"},
		},
		"attachment/update": {
			OperationID:    "updateAttachment",
			Method:         "POST",
			PathTemplate:   "/content/{id}/child/attachment/{attachmentId}/data",
			RequiredParams: []string{"id", "attachmentId", "file"},
			OptionalParams: []string{"comment", "minorEdit"},
		},
		"attachment/delete": {
			OperationID:    "deleteAttachment",
			Method:         "DELETE",
			PathTemplate:   "/content/{id}/child/attachment/{attachmentId}",
			RequiredParams: []string{"id", "attachmentId"},
		},
		"attachment/download": {
			OperationID:    "downloadAttachment",
			Method:         "GET",
			PathTemplate:   "/content/{id}/child/attachment/{attachmentId}/download",
			RequiredParams: []string{"id", "attachmentId"},
			OptionalParams: []string{"version"},
		},

		// Comment operations
		"comment/list": {
			OperationID:    "getComments",
			Method:         "GET",
			PathTemplate:   "/content/{id}/child/comment",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"expand", "parentVersion", "limit", "start", "location", "depth"},
		},
		"comment/get": {
			OperationID:    "getComment",
			Method:         "GET",
			PathTemplate:   "/content/{id}",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"expand", "version"},
		},
		"comment/create": {
			OperationID:    "createComment",
			Method:         "POST",
			PathTemplate:   "/content/{id}/child/comment",
			RequiredParams: []string{"id", "body"},
			OptionalParams: []string{"location", "inline"},
		},
		"comment/update": {
			OperationID:    "updateComment",
			Method:         "PUT",
			PathTemplate:   "/content/{id}",
			RequiredParams: []string{"id", "version", "body"},
			OptionalParams: []string{},
		},
		"comment/delete": {
			OperationID:    "deleteComment",
			Method:         "DELETE",
			PathTemplate:   "/content/{id}",
			RequiredParams: []string{"id"},
		},

		// Label operations
		"label/list": {
			OperationID:    "getLabels",
			Method:         "GET",
			PathTemplate:   "/content/{id}/label",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"prefix", "limit", "start"},
		},
		"label/add": {
			OperationID:    "addLabels",
			Method:         "POST",
			PathTemplate:   "/content/{id}/label",
			RequiredParams: []string{"id", "labels"},
			OptionalParams: []string{},
		},
		"label/remove": {
			OperationID:    "removeLabel",
			Method:         "DELETE",
			PathTemplate:   "/content/{id}/label/{labelName}",
			RequiredParams: []string{"id", "labelName"},
		},
		"label/search": {
			OperationID:    "getLabeledContent",
			Method:         "GET",
			PathTemplate:   "/label/{labelName}/content",
			RequiredParams: []string{"labelName"},
			OptionalParams: []string{"type", "limit", "start"},
		},

		// User and Group operations
		"user/list": {
			OperationID:    "getUsers",
			Method:         "GET",
			PathTemplate:   "/user",
			RequiredParams: []string{},
			OptionalParams: []string{"accountId", "username", "key", "expand"},
		},
		"user/get": {
			OperationID:    "getUser",
			Method:         "GET",
			PathTemplate:   "/user",
			RequiredParams: []string{"accountId"},
			OptionalParams: []string{"expand"},
		},
		"user/current": {
			OperationID:    "getCurrentUser",
			Method:         "GET",
			PathTemplate:   "/user/current",
			RequiredParams: []string{},
			OptionalParams: []string{"expand"},
		},
		"user/watch": {
			OperationID:    "watchContent",
			Method:         "POST",
			PathTemplate:   "/user/watch/content/{contentId}",
			RequiredParams: []string{"contentId", "accountId"},
		},
		"user/unwatch": {
			OperationID:    "unwatchContent",
			Method:         "DELETE",
			PathTemplate:   "/user/watch/content/{contentId}",
			RequiredParams: []string{"contentId", "accountId"},
		},

		// Permissions operations
		"permission/check": {
			OperationID:    "checkContentPermission",
			Method:         "POST",
			PathTemplate:   "/content/{id}/permission/check",
			RequiredParams: []string{"id", "subject", "operation"},
			OptionalParams: []string{},
		},
		"permission/list": {
			OperationID:    "getContentPermissions",
			Method:         "GET",
			PathTemplate:   "/content/{id}/restriction",
			RequiredParams: []string{"id"},
			OptionalParams: []string{"expand", "limit", "start"},
		},

		// Template operations
		"template/list": {
			OperationID:    "getContentTemplates",
			Method:         "GET",
			PathTemplate:   "/template/page",
			RequiredParams: []string{},
			OptionalParams: []string{"spaceKey", "expand", "limit", "start"},
		},
		"template/get": {
			OperationID:    "getContentTemplate",
			Method:         "GET",
			PathTemplate:   "/template/{contentTemplateId}",
			RequiredParams: []string{"contentTemplateId"},
			OptionalParams: []string{},
		},

		// Macro operations
		"macro/get": {
			OperationID:    "getMacroBodyByHash",
			Method:         "GET",
			PathTemplate:   "/content/{id}/history/{version}/macro/hash/{hash}",
			RequiredParams: []string{"id", "version", "hash"},
		},
		"macro/list": {
			OperationID:    "getContentMacros",
			Method:         "GET",
			PathTemplate:   "/content/{id}/history/{version}/macro/id/{macroId}",
			RequiredParams: []string{"id", "version", "macroId"},
		},

		// Settings operations
		"settings/theme": {
			OperationID:    "getSpaceTheme",
			Method:         "GET",
			PathTemplate:   "/space/{spaceKey}/theme",
			RequiredParams: []string{"spaceKey"},
		},
		"settings/lookandfeel": {
			OperationID:    "getLookAndFeel",
			Method:         "GET",
			PathTemplate:   "/settings/lookandfeel",
			RequiredParams: []string{},
			OptionalParams: []string{"spaceKey"},
		},

		// Audit operations
		"audit/list": {
			OperationID:    "getAuditRecords",
			Method:         "GET",
			PathTemplate:   "/audit",
			RequiredParams: []string{},
			OptionalParams: []string{"startDate", "endDate", "searchString", "limit", "start"},
		},
		"audit/retention": {
			OperationID:    "getAuditRetention",
			Method:         "GET",
			PathTemplate:   "/audit/retention",
			RequiredParams: []string{},
		},
	}
}

// GetEnabledModules returns the list of enabled Confluence modules
func (p *ConfluenceProvider) GetEnabledModules() []string {
	return []string{
		"content",    // Page and blog post management
		"space",      // Space management
		"attachment", // File attachments
		"comment",    // Comments and discussions
		"label",      // Content labeling
		"user",       // User management
		"permission", // Permission and restriction management
		"template",   // Content templates
		"macro",      // Macro management
		"settings",   // Space and global settings
		"audit",      // Audit logging
		"search",     // CQL search
	}
}
