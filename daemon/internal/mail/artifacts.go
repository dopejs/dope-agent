package mail

import "time"

func ThreadArtifact(operation Operation, item ThreadSnapshot) Artifact {
	copy := item
	return Artifact{
		ArtifactID:       newID("mail_artifact"),
		OperationID:      operation.OperationID,
		Kind:             ArtifactKindThreadSnapshot,
		IntegrationID:    operation.IntegrationID,
		EnvironmentScope: operation.EnvironmentScope,
		ThreadID:         item.ThreadID,
		Thread:           &copy,
		CreatedAt:        time.Now().UTC(),
	}
}

func MessageArtifact(operation Operation, item MessageSnapshot) Artifact {
	copy := item
	return Artifact{
		ArtifactID:       newID("mail_artifact"),
		OperationID:      operation.OperationID,
		Kind:             ArtifactKindMessageSnapshot,
		IntegrationID:    operation.IntegrationID,
		EnvironmentScope: operation.EnvironmentScope,
		ThreadID:         item.ThreadID,
		MessageID:        item.MessageID,
		Message:          &copy,
		CreatedAt:        time.Now().UTC(),
	}
}

func DraftArtifact(operation Operation, item DraftSnapshot) Artifact {
	copy := item
	return Artifact{
		ArtifactID:       newID("mail_artifact"),
		OperationID:      operation.OperationID,
		Kind:             ArtifactKindDraftSnapshot,
		IntegrationID:    operation.IntegrationID,
		EnvironmentScope: operation.EnvironmentScope,
		ThreadID:         item.ThreadID,
		DraftID:          item.DraftID,
		Draft:            &copy,
		CreatedAt:        time.Now().UTC(),
	}
}

func AttachmentArtifact(operation Operation, item AttachmentReference) Artifact {
	copy := item
	return Artifact{
		ArtifactID:       newID("mail_artifact"),
		OperationID:      operation.OperationID,
		Kind:             ArtifactKindAttachmentRef,
		IntegrationID:    operation.IntegrationID,
		EnvironmentScope: operation.EnvironmentScope,
		MessageID:        chooseAttachmentMessageID(item),
		DraftID:          chooseAttachmentDraftID(item),
		AttachmentRefID:  item.AttachmentRefID,
		Attachment:       &copy,
		CreatedAt:        time.Now().UTC(),
	}
}

func chooseAttachmentMessageID(item AttachmentReference) string {
	if item.ParentKind == "message" {
		return item.ParentID
	}
	return ""
}

func chooseAttachmentDraftID(item AttachmentReference) string {
	if item.ParentKind == "draft" {
		return item.ParentID
	}
	return ""
}
