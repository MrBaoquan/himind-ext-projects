package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"himind-plugin/andx-custom-page-tools/internal/himindjsonrpc"
)

// The host performed only declaration-scoped transport and removed secrets.
// This extension owns all AndX schemas, identity rules and API semantics.
func verifyAccessImport(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in struct {
		CredentialID string `json:"credential_id"`
		Document     struct {
			Schema    string `json:"schema"`
			Developer string `json:"developer"`
		} `json:"document"`
		Public   map[string]string `json:"public_fields"`
		Response struct {
			Code *int `json:"code"`
			Data struct {
				Developer struct {
					ID     string `json:"id"`
					Handle string `json:"handle"`
				} `json:"developer"`
			} `json:"data"`
		} `json:"response"`
	}
	if json.Unmarshal(request.Params, &in) != nil || in.CredentialID != "andx.developer" || in.Document.Schema != "andx.developer.access.v1" || in.Response.Code == nil || *in.Response.Code != 0 || !andxID.MatchString(in.Response.Data.Developer.ID) || in.Document.Developer != "" && in.Document.Developer != in.Response.Data.Developer.Handle || in.Public["endpoint"] == "" {
		return nil, operationError(errors.New("AndX 开发者身份与接入信息不符"))
	}
	return map[string]any{"public_fields": map[string]string{"endpoint": in.Public["endpoint"], "developer_id": in.Response.Data.Developer.ID}, "identity": in.Response.Data.Developer.ID}, nil
}

func accessOperation(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if err := himindjsonrpc.DecodeParams(request, &in); err != nil {
		return nil, err
	}
	if in.CredentialRef == "" {
		return nil, operationError(errors.New("请先粘贴后台复制的完整开发接入信息"))
	}
	profile, _, err := acquireDeveloperProfile(in, request.Method)
	if err != nil {
		return nil, operationError(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var result map[string]any
	switch request.Method {
	case "andx.page.access.directory":
		path := "/api/developer/self"
		if in.ProjectID != "" {
			if !andxID.MatchString(in.ProjectID) || in.AfterID != "" && !andxID.MatchString(in.AfterID) {
				return nil, operationError(errors.New("AndX 项目或游标无效"))
			}
			path = "/api/developer/projects/" + in.ProjectID + "/exhibits"
			if in.AfterID != "" {
				path += "?afterId=" + in.AfterID
			}
		}
		err = developerRequest(ctx, profile, http.MethodGet, path, nil, "", &result)
	case "andx.page.exhibit.create":
		if !andxID.MatchString(in.ProjectID) || strings.TrimSpace(in.Name) == "" || len([]rune(in.Name)) > 64 {
			return nil, operationError(errors.New("AndX 项目或展项名称无效"))
		}
		body, _ := json.Marshal(map[string]string{"name": strings.TrimSpace(in.Name)})
		err = developerRequest(ctx, profile, http.MethodPost, "/api/developer/projects/"+in.ProjectID+"/exhibits", body, "application/json", &result)
	case "andx.page.workspace.bind", "andx.page.workspace.resolve":
		if !andxID.MatchString(in.ProjectID) || !andxID.MatchString(in.ExhibitID) {
			return nil, operationError(errors.New("必须选择明确的 AndX 项目和展项"))
		}
		err = developerRequest(ctx, profile, http.MethodGet, developerBase(in)+"/context", nil, "", &result)
		if err == nil {
			project, _ := result["project"].(map[string]any)
			exhibit, _ := result["exhibit"].(map[string]any)
			if project["id"] != in.ProjectID || exhibit["id"] != in.ExhibitID {
				return nil, operationError(errors.New("AndX 返回目标与工程绑定不符"))
			}
			return map[string]any{"binding_target": map[string]string{"project_id": in.ProjectID, "exhibit_id": in.ExhibitID, "page_key": "exhibit-" + in.ExhibitID}, "target": result, "project_id": in.ProjectID, "exhibit_id": in.ExhibitID, "page_key": "exhibit-" + in.ExhibitID, "credential_ref": in.CredentialRef, "binding_revision": in.BindingRevision}, nil
		}
	default:
		return nil, operationError(errors.New("未知开发接入操作"))
	}
	if err != nil {
		return nil, operationError(err)
	}
	return result, nil
}
