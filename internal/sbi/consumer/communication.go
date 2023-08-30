package consumer

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/openapi"
	"github.com/free5gc/openapi/Namf_Communication"
	"github.com/free5gc/openapi/models"
)

func BuildUeContextCreateData(ue *amf_context.AmfUe, targetRanId models.NgRanTargetId,
	sourceToTargetData models.N2InfoContent, pduSessionList []models.N2SmInformation,
	n2NotifyUri string, ngapCause *models.NgApCause,
) models.UeContextCreateData {
	var ueContextCreateData models.UeContextCreateData

	ueContext := BuildUeContextModel(ue)
	ueContextCreateData.UeContext = &ueContext
	ueContextCreateData.TargetId = &targetRanId
	ueContextCreateData.SourceToTargetData = &sourceToTargetData
	ueContextCreateData.PduSessionList = pduSessionList
	ueContextCreateData.N2NotifyUri = n2NotifyUri

	if ue.UeRadioCapability != "" {
		ueContextCreateData.UeRadioCapability = &models.N2InfoContent{
			NgapData: &models.RefToBinaryData{
				ContentId: ue.UeRadioCapability,
			},
		}
	}
	ueContextCreateData.NgapCause = ngapCause
	return ueContextCreateData
}

func BuildUeContextModel(ue *amf_context.AmfUe) (ueContext models.UeContext) {
	ueContext.Supi = ue.Supi
	ueContext.SupiUnauthInd = ue.UnauthenticatedSupi

	if ue.Gpsi != "" {
		ueContext.GpsiList = append(ueContext.GpsiList, ue.Gpsi)
	}

	if ue.Pei != "" {
		ueContext.Pei = ue.Pei
	}

	if ue.UdmGroupId != "" {
		ueContext.UdmGroupId = ue.UdmGroupId
	}

	if ue.AusfGroupId != "" {
		ueContext.AusfGroupId = ue.AusfGroupId
	}

	if ue.RoutingIndicator != "" {
		ueContext.RoutingIndicator = ue.RoutingIndicator
	}

	if ue.AccessAndMobilitySubscriptionData != nil {
		if ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr != nil {
			ueContext.SubUeAmbr = &models.Ambr{
				Uplink:   ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr.Uplink,
				Downlink: ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr.Downlink,
			}
		}
		if ue.AccessAndMobilitySubscriptionData.RfspIndex != 0 {
			ueContext.SubRfsp = ue.AccessAndMobilitySubscriptionData.RfspIndex
		}
	}

	if ue.PcfId != "" {
		ueContext.PcfId = ue.PcfId
	}

	if ue.AmPolicyUri != "" {
		ueContext.PcfAmPolicyUri = ue.AmPolicyUri
	}

	if ue.AmPolicyAssociation != nil {
		if len(ue.AmPolicyAssociation.Triggers) > 0 {
			ueContext.AmPolicyReqTriggerList = buildAmPolicyReqTriggers(ue.AmPolicyAssociation.Triggers)
		}
	}

	for _, eventSub := range ue.EventSubscriptionsInfo {
		if eventSub.EventSubscription != nil {
			ueContext.EventSubscriptionList = append(ueContext.EventSubscriptionList, *eventSub.EventSubscription)
		}
	}

	if ue.TraceData != nil {
		ueContext.TraceData = ue.TraceData
	}
	return ueContext
}

func buildAmPolicyReqTriggers(triggers []models.RequestTrigger) (amPolicyReqTriggers []models.AmPolicyReqTrigger) {
	for _, trigger := range triggers {
		switch trigger {
		case models.RequestTrigger_LOC_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTrigger_LOCATION_CHANGE)
		case models.RequestTrigger_PRA_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTrigger_PRA_CHANGE)
		case models.RequestTrigger_SERV_AREA_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTrigger_SARI_CHANGE)
		case models.RequestTrigger_RFSP_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTrigger_RFSP_INDEX_CHANGE)
		}
	}
	return
}

func CreateUEContextRequest(ue *amf_context.AmfUe, ueContextCreateData models.UeContextCreateData) (
	ueContextCreatedData *models.UeContextCreatedData, problemDetails *models.ProblemDetails, err error,
) {
	configuration := Namf_Communication.NewConfiguration()
	configuration.SetBasePath(ue.TargetAmfUri)
	client := Namf_Communication.NewAPIClient(configuration)

	req := models.CreateUeContextRequest{
		JsonData: &ueContextCreateData,
	}
	res, httpResp, localErr := client.IndividualUeContextDocumentApi.CreateUEContext(context.TODO(), ue.Guti, req)
	defer func() {
		if httpResp != nil {
			if rspCloseErr := httpResp.Body.Close(); rspCloseErr != nil {
				logger.ConsumerLog.Errorf("CreateUEContext response body cannot close: %+v",
					rspCloseErr)
			}
		}
	}()
	if localErr == nil {
		ueContextCreatedData = res.JsonData
		logger.ConsumerLog.Debugf("UeContextCreatedData: %+v", *ueContextCreatedData)
	} else if httpResp != nil {
		if httpResp.Status != localErr.Error() {
			err = localErr
			return
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("%s: server no response", ue.TargetAmfUri)
	}
	return ueContextCreatedData, problemDetails, err
}

func ReleaseUEContextRequest(ue *amf_context.AmfUe, ngapCause models.NgApCause) (
	problemDetails *models.ProblemDetails, err error,
) {
	configuration := Namf_Communication.NewConfiguration()
	configuration.SetBasePath(ue.TargetAmfUri)
	client := Namf_Communication.NewAPIClient(configuration)

	var ueContextId string
	if ue.Supi != "" {
		ueContextId = ue.Supi
	} else {
		ueContextId = ue.Pei
	}

	ueContextRelease := models.UeContextRelease{
		NgapCause: &ngapCause,
	}
	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSEmergencyRegistration && ue.UnauthenticatedSupi {
		ueContextRelease.Supi = ue.Supi
		ueContextRelease.UnauthenticatedSupi = true
	}

	httpResp, localErr := client.IndividualUeContextDocumentApi.ReleaseUEContext(
		context.TODO(), ueContextId, ueContextRelease)
	defer func() {
		if httpResp != nil {
			if rspCloseErr := httpResp.Body.Close(); rspCloseErr != nil {
				logger.ConsumerLog.Errorf("ReleaseUEContext response body cannot close: %+v",
					rspCloseErr)
			}
		}
	}()
	if localErr == nil {
		return
	} else if httpResp != nil {
		if httpResp.Status != localErr.Error() {
			err = localErr
			return
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("%s: server no response", ue.TargetAmfUri)
	}
	return problemDetails, err
}

func UEContextTransferRequest(
	ue *amf_context.AmfUe, accessType models.AccessType, transferReason models.TransferReason) (
	ueContextTransferRspData *models.UeContextTransferRspData, problemDetails *models.ProblemDetails, err error,
) {
	configuration := Namf_Communication.NewConfiguration()
	configuration.SetBasePath(ue.TargetAmfUri)
	client := Namf_Communication.NewAPIClient(configuration)

	ueContextTransferReqData := models.UeContextTransferReqData{
		Reason:     transferReason,
		AccessType: accessType,
	}

	req := models.UeContextTransferRequest{
		JsonData: &ueContextTransferReqData,
	}
	if transferReason == models.TransferReason_INIT_REG || transferReason == models.TransferReason_MOBI_REG {
		var buf bytes.Buffer
		if err := binary.Write(&buf, binary.BigEndian, ue.SecurityHeader.ProtocolDiscriminator); err != nil {
			logger.ConsumerLog.Error("NAS encode error (SecurityHeader/ProtocolDiscriminator): %w", err)
		}
		// TODO: check the correct security header
		ue.SecurityHeader.SecurityHeaderType = 2 & 0x0f
		if err := binary.Write(&buf, binary.BigEndian, ue.SecurityHeader.SecurityHeaderType); err != nil {
			logger.ConsumerLog.Error("NAS encode error (SecurityHeader/SecurityHeaderType): %w", err)
		}
		if err := binary.Write(&buf, binary.BigEndian, ue.SecurityHeader.MessageAuthenticationCode); err != nil {
			logger.ConsumerLog.Error("NAS encode error (SecurityHeader/MessageAuthenticationCode): %w", err)
		}
		if err := binary.Write(&buf, binary.BigEndian, ue.SecurityHeader.SequenceNumber); err != nil {
			logger.ConsumerLog.Error("NAS encode error (SecurityHeader/SequenceNumber): %w", err)
		}
		err = ue.RegistrationRequest.EncodeRegistrationRequest(&buf)
		if err != nil {
			return nil, nil, fmt.Errorf("re-encoding registration request message is failed: %w", err)
		}
		ueContextTransferReqData.RegRequest = &models.N1MessageContainer{
			N1MessageClass: models.N1MessageClass__5_GMM,
			N1MessageContent: &models.RefToBinaryData{
				ContentId: "n1Msg",
			},
		}
		req.BinaryDataN1Message = buf.Bytes()
	}

	// guti format is defined at TS 29.518 Table 6.1.3.2.2-1 5g-guti-[0-9]{5,6}[0-9a-fA-F]{14}
	ueContextId := fmt.Sprintf("5g-guti-%s", ue.Guti)

	res, httpResp, localErr := client.IndividualUeContextDocumentApi.UEContextTransfer(context.TODO(), ueContextId, req)
	defer func() {
		if httpResp != nil {
			if rspCloseErr := httpResp.Body.Close(); rspCloseErr != nil {
				logger.ConsumerLog.Errorf("UEContextTransfer response body cannot close: %+v",
					rspCloseErr)
			}
		}
	}()
	if localErr == nil {
		ueContextTransferRspData = res.JsonData
		logger.ConsumerLog.Debugf("UeContextTransferRspData: %+v", *ueContextTransferRspData)
	} else if httpResp != nil {
		if httpResp.Status != localErr.Error() {
			err = localErr
			return
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("%s: server no response", ue.TargetAmfUri)
	}
	return ueContextTransferRspData, problemDetails, err
}

// This operation is called "RegistrationCompleteNotify" at TS 23.502
func RegistrationStatusUpdate(ue *amf_context.AmfUe, request models.UeRegStatusUpdateReqData) (
	regStatusTransferComplete bool, problemDetails *models.ProblemDetails, err error,
) {
	configuration := Namf_Communication.NewConfiguration()
	configuration.SetBasePath(ue.TargetAmfUri)
	client := Namf_Communication.NewAPIClient(configuration)

	ueContextId := fmt.Sprintf("5g-guti-%s", ue.Guti)
	res, httpResp, localErr := client.IndividualUeContextDocumentApi.
		RegistrationStatusUpdate(context.TODO(), ueContextId, request)
	defer func() {
		if httpResp != nil {
			if rspCloseErr := httpResp.Body.Close(); rspCloseErr != nil {
				logger.ConsumerLog.Errorf("RegistrationStatusUpdate response body cannot close: %+v",
					rspCloseErr)
			}
		}
	}()
	if localErr == nil {
		regStatusTransferComplete = res.RegStatusTransferComplete
	} else if httpResp != nil {
		if httpResp.Status != localErr.Error() {
			err = localErr
			return
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("%s: server no response", ue.TargetAmfUri)
	}
	return regStatusTransferComplete, problemDetails, err
}
