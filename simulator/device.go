package simulator

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/chirpstack-simulator/internal/cpuinfo"
	"github.com/brocaar/lorawan"
	"github.com/chirpstack/chirpstack/api/go/v4/gw"
)

// DeviceOption is the interface for a device option.
type DeviceOption func(*Device) error

type deviceState int

const (
	deviceStateOTAA deviceState = iota
	deviceStateActivated
)

// Device contains the state of a simulated LoRaWAN OTAA device (1.0.x).
type Device struct {
	sync.RWMutex

	// Context to cancel device.
	ctx context.Context

	// Cancel function.
	cancel context.CancelFunc

	// Waitgroup to wait until simulation has been fully cancelled.
	wg *sync.WaitGroup

	// DevEUI.
	devEUI lorawan.EUI64

	// JoinEUI.
	joinEUI lorawan.EUI64

	// AppKey.
	appKey lorawan.AES128Key

	// Processor ID for CPU info payload
	processorID string

	// Interval in which device sends uplinks.
	uplinkInterval time.Duration

	// Total number of uplinks to send, before terminating.
	uplinkCount uint32

	// Device sends uplink as confirmed.
	confirmed bool

	// Payload (plaintext) which the device sends as uplink.
	payload []byte

	// FPort used for sending uplinks.
	fPort uint8

	// Assigned device address.
	devAddr lorawan.DevAddr

	// DevNonce.
	devNonce lorawan.DevNonce

	// Uplink frame-counter.
	fCntUp uint32

	// Downlink frame-counter.
	fCntDown uint32

	// Application session-key.
	appSKey lorawan.AES128Key

	// Network session-key.
	nwkSKey lorawan.AES128Key

	// Activation state.
	state deviceState

	// Downlink frames channel (used by the gateway). Note that the gateway
	// forwards downlink frames to all associated devices, as only the device
	// is able to validate the addressee.
	downlinkFrames chan gw.DownlinkFrame

	// The associated gateway through which the device simulates its uplinks.
	gateways []*Gateway

	// Random DevNonce
	randomDevNonce bool

	// TXInfo for uplink
	uplinkTXInfo gw.UplinkTxInfo

	// Downlink handler function.
	downlinkHandlerFunc func(confirmed, ack bool, fCntDown uint32, fPort uint8, data []byte) error

	// OTAA delay.
	otaaDelay time.Duration
}

// WithAppKey sets the AppKey.
func WithAppKey(appKey lorawan.AES128Key) DeviceOption {
	return func(d *Device) error {
		d.appKey = appKey
		return nil
	}
}

// WithDevEUI sets the DevEUI.
func WithDevEUI(devEUI lorawan.EUI64) DeviceOption {
	return func(d *Device) error {
		d.devEUI = devEUI
		return nil
	}
}

// WithJoinEUI sets the JoinEUI.
func WithJoinEUI(joinEUI lorawan.EUI64) DeviceOption {
	return func(d *Device) error {
		d.joinEUI = joinEUI
		return nil
	}
}

// WithProcessorID sets the processor ID for CPU info payload.
func WithProcessorID(processorID string) DeviceOption {
	return func(d *Device) error {
		log.WithFields(log.Fields{
			"dev_eui":          d.devEUI,
			"old_processor_id": d.processorID,
			"new_processor_id": processorID,
			"type":             fmt.Sprintf("%T", processorID),
			"len":              len(processorID),
			"is_empty":         processorID == "",
			"is_none":          processorID == "none",
		}).Info("simulator: setting processor ID")

		d.processorID = processorID
		return nil
	}
}

// WithOTAADelay sets the OTAA delay.
func WithOTAADelay(delay time.Duration) DeviceOption {
	return func(d *Device) error {
		d.otaaDelay = delay
		return nil
	}
}

// WithUplinkInterval sets the uplink interval.
func WithUplinkInterval(interval time.Duration) DeviceOption {
	return func(d *Device) error {
		d.uplinkInterval = interval
		return nil
	}
}

// WithUplinkCount sets the uplink count, after which the device simulation
// ends.
func WithUplinkCount(count uint32) DeviceOption {
	return func(d *Device) error {
		d.uplinkCount = count
		return nil
	}
}

// WithUplinkPayload sets the uplink payload.
func WithUplinkPayload(confirmed bool, fPort uint8, pl []byte) DeviceOption {
	return func(d *Device) error {
		d.fPort = fPort
		d.payload = pl
		d.confirmed = confirmed
		return nil
	}
}

// WithGateways adds the device to the given gateways.
// Use this function after WithDevEUI!
func WithGateways(gws []*Gateway) DeviceOption {
	return func(d *Device) error {
		d.gateways = gws

		for i := range d.gateways {
			d.gateways[i].addDevice(d.devEUI, d.downlinkFrames)
		}
		return nil
	}
}

// WithRandomDevNonce randomizes the OTAA DevNonce instead of using a counter value.
func WithRandomDevNonce() DeviceOption {
	return func(d *Device) error {
		d.randomDevNonce = true
		return nil
	}
}

// WithUplinkTXInfo sets the TXInfo used for simulating the uplinks.
func WithUplinkTXInfo(txInfo gw.UplinkTxInfo) DeviceOption {
	return func(d *Device) error {
		d.uplinkTXInfo = txInfo
		return nil
	}
}

// WithDownlinkHandlerFunc sets the downlink handler func.
func WithDownlinkHandlerFunc(f func(confirmed, ack bool, fCntDown uint32, fPort uint8, data []byte) error) DeviceOption {
	return func(d *Device) error {
		d.downlinkHandlerFunc = f
		return nil
	}
}

// NewDevice creates a new device simulation.
func NewDevice(ctx context.Context, wg *sync.WaitGroup, opts ...DeviceOption) (*Device, error) {
	ctx, cancel := context.WithCancel(ctx)

	d := &Device{
		ctx:    ctx,
		cancel: cancel,
		wg:     wg,

		downlinkFrames: make(chan gw.DownlinkFrame, 100),
		state:          deviceStateOTAA,
	}

	for _, o := range opts {
		if err := o(d); err != nil {
			return nil, err
		}
	}

	log.WithFields(log.Fields{
		"dev_eui":      d.devEUI,
		"processor_id": d.processorID,
		"type":         fmt.Sprintf("%T", d.processorID),
		"len":          len(d.processorID),
		"is_empty":     d.processorID == "",
		"is_none":      d.processorID == "none",
	}).Info("simulator: new otaa device created")

	wg.Add(2)

	go d.uplinkLoop()
	go d.downlinkLoop()

	return d, nil
}

// uplinkLoop first handle the OTAA activation, after which it will periodically
// sends an uplink with the configured payload and fport.
func (d *Device) uplinkLoop() {
	defer func() {
		fmt.Printf("DEBUG: Device %s uplinkLoop defer called\n", d.devEUI)
		d.cancel()
		d.wg.Done()
		fmt.Printf("DEBUG: Device %s uplinkLoop completed\n", d.devEUI)
	}()

	var cancelled bool
	go func() {
		<-d.ctx.Done()
		fmt.Printf("DEBUG: Device %s context cancelled, setting cancelled=true\n", d.devEUI)
		cancelled = true
	}()

	log.WithField("dev_eui", d.devEUI).Info("simulator: device uplink loop started, waiting for OTAA delay")
	time.Sleep(d.otaaDelay)
	log.WithField("dev_eui", d.devEUI).Info("simulator: OTAA delay completed, starting device loop")
	fmt.Printf("DEBUG: Device %s entering main loop\n", d.devEUI)

	for !cancelled {
		fmt.Printf("DEBUG: Device %s loop iteration, cancelled=%v\n", d.devEUI, cancelled)
		currentState := d.getState()
		fmt.Printf("DEBUG: Device %s current state: %d\n", d.devEUI, currentState)

		switch currentState {
		case deviceStateOTAA:
			fmt.Printf("DEBUG: Device %s is about to send join request\n", d.devEUI)
			log.WithField("dev_eui", d.devEUI).Info("simulator: device in OTAA state, sending join request")
			d.joinRequest()
			fmt.Printf("DEBUG: Device %s join request completed\n", d.devEUI)
			log.WithField("dev_eui", d.devEUI).Info("simulator: join request sent, waiting 6 seconds")
			time.Sleep(6 * time.Second)
		case deviceStateActivated:
			fmt.Printf("DEBUG: Device %s is activated, sending data uplink\n", d.devEUI)
			log.WithField("dev_eui", d.devEUI).Debug("simulator: device activated, sending data uplink")
			d.dataUp()

			if d.uplinkCount != 0 {
				if d.fCntUp >= d.uplinkCount {
					// d.cancel() also cancels the downlink loop. Wait one
					// second in order to process any potential downlink
					// response (e.g. and ack).
					time.Sleep(time.Second)
					d.cancel()
					return
				}
			}

			time.Sleep(d.uplinkInterval)
		default:
			fmt.Printf("DEBUG: Device %s unknown state: %d\n", d.devEUI, currentState)
		}
	}

	fmt.Printf("DEBUG: Device %s main loop completed\n", d.devEUI)
}

// downlinkLoop handles the downlink messages.
// Note: as a gateway does not know the addressee of the downlink, it is up to
// the handling functions to validate the MIC etc..
func (d *Device) downlinkLoop() {
	defer d.cancel()
	defer d.wg.Done()

	log.WithField("dev_eui", d.devEUI).Info("simulator: device downlink loop started")

	for {
		select {
		case <-d.ctx.Done():
			log.WithField("dev_eui", d.devEUI).Info("simulator: device downlink loop context cancelled")
			return

		case pl := <-d.downlinkFrames:
			log.WithFields(log.Fields{
				"dev_eui": d.devEUI,
				"frames":  len(pl.Items),
			}).Info("simulator: device received downlink frame")

			for i, item := range pl.Items {
				log.WithFields(log.Fields{
					"dev_eui": d.devEUI,
					"frame":   i,
					"size":    len(item.PhyPayload),
				}).Debug("simulator: processing downlink frame item")

				err := func() error {
					var phy lorawan.PHYPayload

					if err := phy.UnmarshalBinary(item.PhyPayload); err != nil {
						return errors.Wrap(err, "unmarshal phypayload error")
					}

					log.WithFields(log.Fields{
						"dev_eui": d.devEUI,
						"mtype":   phy.MHDR.MType,
						"major":   phy.MHDR.Major,
					}).Info("simulator: device processing downlink message")

					switch phy.MHDR.MType {
					case lorawan.JoinAccept:
						log.WithField("dev_eui", d.devEUI).Info("simulator: device received JoinAccept message")
						return d.joinAccept(phy)
					case lorawan.UnconfirmedDataDown, lorawan.ConfirmedDataDown:
						log.WithField("dev_eui", d.devEUI).Info("simulator: device received DataDown message")
						return d.downlinkData(phy)
					default:
						log.WithFields(log.Fields{
							"dev_eui": d.devEUI,
							"mtype":   phy.MHDR.MType,
						}).Warn("simulator: device received unknown message type")
					}

					return nil
				}()

				if err != nil {
					log.WithError(err).WithField("dev_eui", d.devEUI).Error("simulator: device downlink processing error")
				}
			}
		}
	}
}

// joinRequest sends the join-request.
func (d *Device) joinRequest() {
	fmt.Printf("DEBUG: joinRequest called for device %s\n", d.devEUI)

	log.WithFields(log.Fields{
		"dev_eui": d.devEUI,
	}).Info("simulator: send OTAA request")

	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			MType: lorawan.JoinRequest,
			Major: lorawan.LoRaWANR1,
		},
		MACPayload: &lorawan.JoinRequestPayload{
			DevEUI:   d.devEUI,
			JoinEUI:  d.joinEUI,
			DevNonce: d.getDevNonce(),
		},
	}

	log.WithFields(log.Fields{
		"dev_eui":   d.devEUI,
		"join_eui":  d.joinEUI,
		"dev_nonce": d.devNonce,
	}).Debug("simulator: join request payload created")

	if err := phy.SetUplinkJoinMIC(d.appKey); err != nil {
		log.WithError(err).Error("simulator: set uplink join mic error")
		return
	}

	log.WithField("dev_eui", d.devEUI).Debug("simulator: join request MIC set, sending uplink")
	d.sendUplink(phy)
	log.WithField("dev_eui", d.devEUI).Info("simulator: join request sent successfully")

	deviceJoinRequestCounter().Inc()
}

// generateCPUPayload генерирует payload на основе CPU info
func (d *Device) generateCPUPayload() []byte {
	log.WithFields(log.Fields{
		"dev_eui":      d.devEUI,
		"processor_id": d.processorID,
		"type":         fmt.Sprintf("%T", d.processorID),
		"len":          len(d.processorID),
		"is_empty":     d.processorID == "",
		"is_none":      d.processorID == "none",
	}).Info("simulator: generating CPU payload")

	// Если processorID не установлен или равен "none", используем значения по умолчанию
	if d.processorID == "" || d.processorID == "none" {
		log.WithFields(log.Fields{
			"dev_eui":      d.devEUI,
			"processor_id": d.processorID,
		}).Info("simulator: using default payload (no processor ID)")
		return d.generateDefaultPayload()
	}

	// Пытаемся получить CPU info для указанного процессора
	processorID, err := strconv.ParseUint(d.processorID, 10, 8)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"dev_eui":      d.devEUI,
			"processor_id": d.processorID,
		}).Warn("simulator: invalid processor ID, using default payload")
		return d.generateDefaultPayload()
	}

	log.WithFields(log.Fields{
		"dev_eui":      d.devEUI,
		"processor_id": d.processorID,
		"parsed_id":    processorID,
	}).Debug("simulator: attempting to get CPU info")

	// Импортируем cpuinfo пакет
	cpuInfo, err := cpuinfo.GetCPUInfo(uint8(processorID))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"dev_eui":      d.devEUI,
			"processor_id": d.processorID,
		}).Warn("simulator: failed to get CPU info, using default payload")
		return d.generateDefaultPayload()
	}

	log.WithFields(log.Fields{
		"dev_eui":      d.devEUI,
		"processor_id": d.processorID,
		"cpu_mhz":      cpuInfo.CPUMHz,
		"vendor_id":    cpuInfo.VendorID,
	}).Info("simulator: successfully generated CPU payload")

	return cpuInfo.ToPayload()
}

// generateDefaultPayload генерирует payload с значениями по умолчанию
func (d *Device) generateDefaultPayload() []byte {
	log.WithFields(log.Fields{
		"dev_eui":      d.devEUI,
		"processor_id": d.processorID,
		"type":         fmt.Sprintf("%T", d.processorID),
		"len":          len(d.processorID),
		"is_empty":     d.processorID == "",
		"is_none":      d.processorID == "none",
	}).Info("simulator: generating default payload")

	// Создаем payload: processor=0, cpu_mhz=0.0, vendor_id="unknown"
	payload := make([]byte, 25)

	// Processor ID - для устройств с processorID = "none" или пустым используем 0
	payload[0] = 0

	// CPU MHz = 0.0 (4 байта, little-endian)
	binary.LittleEndian.PutUint32(payload[1:5], 0)

	// Vendor ID = "unknown" (20 байт)
	copy(payload[5:25], []byte("unknown"))

	log.WithFields(log.Fields{
		"dev_eui":      d.devEUI,
		"processor_id": d.processorID,
		"payload_0":    payload[0],
		"payload_hex":  hex.EncodeToString(payload),
	}).Info("simulator: default payload generated")

	return payload
}

// dataUp sends an data uplink.
func (d *Device) dataUp() {
	log.WithFields(log.Fields{
		"dev_eui":   d.devEUI,
		"dev_addr":  d.devAddr,
		"confirmed": d.confirmed,
	}).Debug("simulator: send uplink data")

	// Генерируем payload на основе CPU info
	payload := d.generateCPUPayload()

	log.WithFields(log.Fields{
		"dev_eui":      d.devEUI,
		"processor_id": d.processorID,
		"type":         fmt.Sprintf("%T", d.processorID),
		"len":          len(d.processorID),
		"is_empty":     d.processorID == "",
		"is_none":      d.processorID == "none",
		"payload_len":  len(payload),
		"payload_0":    payload[0],
		"payload_hex":  hex.EncodeToString(payload),
	}).Info("simulator: generated payload for uplink")

	mType := lorawan.UnconfirmedDataUp
	if d.confirmed {
		mType = lorawan.ConfirmedDataUp
	}

	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			MType: mType,
			Major: lorawan.LoRaWANR1,
		},
		MACPayload: &lorawan.MACPayload{
			FHDR: lorawan.FHDR{
				DevAddr: d.devAddr,
				FCnt:    d.fCntUp,
				FCtrl: lorawan.FCtrl{
					ADR: false,
				},
			},
			FPort: &d.fPort,
			FRMPayload: []lorawan.Payload{
				&lorawan.DataPayload{
					Bytes: payload,
				},
			},
		},
	}

	if err := phy.EncryptFRMPayload(d.appSKey); err != nil {
		log.WithError(err).Error("simulator: encrypt FRMPayload error")
		return
	}

	if err := phy.SetUplinkDataMIC(lorawan.LoRaWAN1_0, 0, 0, 0, d.nwkSKey, d.nwkSKey); err != nil {
		log.WithError(err).Error("simulator: set uplink data mic error")
		return
	}

	d.fCntUp++

	d.sendUplink(phy)

	deviceUplinkCounter().Inc()
}

// joinAccept validates and handles the join-accept downlink.
func (d *Device) joinAccept(phy lorawan.PHYPayload) error {
	log.WithField("dev_eui", d.devEUI).Info("simulator: device processing JoinAccept message")

	err := phy.DecryptJoinAcceptPayload(d.appKey)
	if err != nil {
		log.WithError(err).WithField("dev_eui", d.devEUI).Error("simulator: device failed to decrypt JoinAccept payload")
		return errors.Wrap(err, "decrypt join-accept payload error")
	}
	log.WithField("dev_eui", d.devEUI).Info("simulator: device successfully decrypted JoinAccept payload")

	ok, err := phy.ValidateDownlinkJoinMIC(lorawan.JoinRequestType, d.joinEUI, d.devNonce, d.appKey)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"dev_eui": d.devEUI,
		}).Error("simulator: device failed to validate JoinAccept MIC")
		return errors.Wrap(err, "validate downlink join MIC error")
	}
	if !ok {
		log.WithFields(log.Fields{
			"dev_eui": d.devEUI,
		}).Error("simulator: device JoinAccept MIC validation failed")
		return errors.New("invalid join-accept MIC")
	}
	log.WithField("dev_eui", d.devEUI).Info("simulator: device JoinAccept MIC validation successful")

	jaPL, ok := phy.MACPayload.(*lorawan.JoinAcceptPayload)
	if !ok {
		log.WithField("dev_eui", d.devEUI).Error("simulator: device expected *lorawan.JoinAcceptPayload")
		return errors.New("expected *lorawan.JoinAcceptPayload")
	}

	log.WithFields(log.Fields{
		"dev_eui":     d.devEUI,
		"home_net_id": jaPL.HomeNetID,
		"join_nonce":  jaPL.JoinNonce,
		"dev_nonce":   d.devNonce,
	}).Info("simulator: device extracting session keys from JoinAccept")

	d.appSKey, err = getAppSKey(jaPL.DLSettings.OptNeg, d.appKey, jaPL.HomeNetID, d.joinEUI, jaPL.JoinNonce, d.devNonce)
	if err != nil {
		log.WithError(err).WithField("dev_eui", d.devEUI).Error("simulator: device failed to derive AppSKey")
		return errors.Wrap(err, "get AppSKey error")
	}
	log.WithField("dev_eui", d.devEUI).Info("simulator: device AppSKey derived successfully")

	d.nwkSKey, err = getFNwkSIntKey(jaPL.DLSettings.OptNeg, d.appKey, jaPL.HomeNetID, d.joinEUI, jaPL.JoinNonce, d.devNonce)
	if err != nil {
		log.WithError(err).WithField("dev_eui", d.devEUI).Error("simulator: device failed to derive NwkSKey")
		return errors.Wrap(err, "get NwkSKey error")
	}
	log.WithField("dev_eui", d.devEUI).Info("simulator: device NwkSKey derived successfully")

	d.devAddr = jaPL.DevAddr

	log.WithFields(log.Fields{
		"dev_eui":  d.devEUI,
		"dev_addr": d.devAddr,
	}).Info("simulator: device OTAA activated successfully")

	d.setState(deviceStateActivated)
	deviceJoinAcceptCounter().Inc()

	return nil
}

// downlinkData validates and handles the downlink data.
func (d *Device) downlinkData(phy lorawan.PHYPayload) error {
	ok, err := phy.ValidateDownlinkDataMIC(lorawan.LoRaWAN1_0, 0, d.nwkSKey)
	if err != nil {
		log.WithFields(log.Fields{
			"dev_eui": d.devEUI,
		}).Debug("simulator: invalid downlink data MIC")
		return nil
	}

	if !ok {
		log.WithFields(log.Fields{
			"dev_eui": d.devEUI,
		}).Debug("simulator: invalid downlink data MIC")
		return nil
	}

	macPL, ok := phy.MACPayload.(*lorawan.MACPayload)
	if !ok {
		return fmt.Errorf("expected *lorawan.MACPayload, got: %T", phy.MACPayload)
	}

	gap := uint32(uint16(macPL.FHDR.FCnt) - uint16(d.fCntDown%(1<<16)))
	d.fCntDown = d.fCntDown + gap

	var data []byte
	var fPort uint8
	if macPL.FPort != nil {
		fPort = *macPL.FPort
	}

	if fPort != 0 {
		err := phy.DecryptFRMPayload(d.appSKey)
		if err != nil {
			return errors.Wrap(err, "decrypt frmpayload error")
		}

		if len(macPL.FRMPayload) != 0 {
			pl, ok := macPL.FRMPayload[0].(*lorawan.DataPayload)
			if !ok {
				return fmt.Errorf("expected *lorawan.DataPayload, got: %T", macPL.FRMPayload[0])
			}

			data = pl.Bytes
		}
	}

	log.WithFields(log.Fields{
		"confirmed": phy.MHDR.MType == lorawan.ConfirmedDataDown,
		"ack":       macPL.FHDR.FCtrl.ACK,
		"f_cnt":     d.fCntDown,
		"dev_eui":   d.devEUI,
		"f_port":    fPort,
		"data":      hex.EncodeToString(data),
	}).Info("simulator: device received downlink data")

	if d.downlinkHandlerFunc == nil {
		return nil
	}

	return d.downlinkHandlerFunc(phy.MHDR.MType == lorawan.ConfirmedDataDown, macPL.FHDR.FCtrl.ACK, d.fCntDown, fPort, data)
}

// sendUplink sends
func (d *Device) sendUplink(phy lorawan.PHYPayload) error {
	b, err := phy.MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "marshal phypayload error")
	}

	pl := gw.UplinkFrame{
		PhyPayload: b,
		TxInfo:     &d.uplinkTXInfo,
	}

	for i := range d.gateways {
		if err := d.gateways[i].SendUplinkFrame(pl); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"dev_eui": d.devEUI,
			}).Error("simulator: send uplink frame error")
		}
	}

	return nil
}

// getDevNonce increments and returns a LoRaWAN DevNonce.
func (d *Device) getDevNonce() lorawan.DevNonce {
	if d.randomDevNonce {
		b := make([]byte, 2)
		_, _ = crand.Read(b)

		d.devNonce = lorawan.DevNonce(binary.BigEndian.Uint16(b))
	} else {
		d.devNonce++
	}

	return d.devNonce
}

// getState returns the current device state.
func (d *Device) getState() deviceState {
	d.RLock()
	defer d.RUnlock()

	return d.state
}

// setState sets the device to the given state.
func (d *Device) setState(state deviceState) {
	d.Lock()
	defer d.Unlock()

	fmt.Printf("DEBUG: Device %s state changing from %d to %d\n", d.devEUI, d.state, state)
	d.state = state
}
