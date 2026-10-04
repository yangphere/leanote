package service

import applicationnotes "github.com/yangphere/leanote/app/application/notes"

type WorkspaceErrorCategory = applicationnotes.ErrorCategory
type WorkspaceItemResult = applicationnotes.ItemResult
type WorkspaceCommandResult = applicationnotes.CommandResult

const (
	WorkspaceValidation   = applicationnotes.ErrorValidation
	WorkspaceNotFound     = applicationnotes.ErrorNotFound
	WorkspaceUnauthorized = applicationnotes.ErrorUnauthorized
	WorkspaceConflict     = applicationnotes.ErrorConflict
	WorkspaceDuplicate    = applicationnotes.ErrorDuplicate
	WorkspaceStorage      = applicationnotes.ErrorStorage
	WorkspaceTimeout      = applicationnotes.ErrorTimeout
	WorkspacePartialWrite = applicationnotes.ErrorPartialWrite
	WorkspaceSideEffect   = applicationnotes.ErrorSideEffect
)
