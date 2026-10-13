package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerCmsTools(add toolAdder) {
	add(
		"magento_list_cms_pages",
		"List CMS static pages in Magento using searchCriteria (pageSize, currentPage, identifier, title).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Page size (default 20)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"identifier":   map[string]interface{}{"type": "string", "description": "Filter by CMS page URL key / identifier (e.g. home, about-us)"},
			"title":        map[string]interface{}{"type": "string", "description": "Filter by CMS page title"},
		}),
		nil,
		handleMagentoListCmsPages,
	)

	add(
		"magento_get_cms_page",
		"Retrieve CMS page details by Page ID.",
		baseProps(map[string]interface{}{
			"page_id": map[string]interface{}{"type": "string", "description": "CMS page entity ID"},
		}),
		[]string{"page_id"},
		handleMagentoGetCmsPage,
	)

	add(
		"magento_list_cms_blocks",
		"List CMS static blocks in Magento using searchCriteria (pageSize, currentPage, identifier, title).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Page size (default 20)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"identifier":   map[string]interface{}{"type": "string", "description": "Filter by CMS block identifier (e.g. footer_links)"},
			"title":        map[string]interface{}{"type": "string", "description": "Filter by CMS block title"},
		}),
		nil,
		handleMagentoListCmsBlocks,
	)

	add(
		"magento_get_cms_block",
		"Retrieve CMS static block details by Block ID.",
		baseProps(map[string]interface{}{
			"block_id": map[string]interface{}{"type": "string", "description": "CMS block entity ID"},
		}),
		[]string{"block_id"},
		handleMagentoGetCmsBlock,
	)
}

func handleMagentoListCmsPages(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageSize := intArg(m, "page_size", 20)
	currentPage := intArg(m, "current_page", 1)

	q := url.Values{}
	fb := newFilterBuilder(q, pageSize, currentPage)

	if id := strArg(m, "identifier"); id != "" {
		fb.add("identifier", "%"+id+"%", "like")
	}
	if title := strArg(m, "title"); title != "" {
		fb.add("title", "%"+title+"%", "like")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/cmsPage/search", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetCmsPage(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageID := strArg(m, "page_id")
	if pageID == "" {
		return mcp.ToolResultError("page_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/cmsPage/%s", pageID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoListCmsBlocks(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageSize := intArg(m, "page_size", 20)
	currentPage := intArg(m, "current_page", 1)

	q := url.Values{}
	fb := newFilterBuilder(q, pageSize, currentPage)

	if id := strArg(m, "identifier"); id != "" {
		fb.add("identifier", "%"+id+"%", "like")
	}
	if title := strArg(m, "title"); title != "" {
		fb.add("title", "%"+title+"%", "like")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/cmsBlock/search", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetCmsBlock(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	blockID := strArg(m, "block_id")
	if blockID == "" {
		return mcp.ToolResultError("block_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/cmsBlock/%s", blockID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
