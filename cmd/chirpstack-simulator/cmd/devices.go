package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/brocaar/chirpstack-simulator/internal/config"
	"github.com/brocaar/chirpstack-simulator/internal/simulator"
)

var (
	deviceFile string
	statusFile string
	action     string
)

// DeviceStatus представляет статус устройства
type DeviceStatus struct {
	DevEUI   string `json:"dev_eui"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
	LastSeen string `json:"last_seen,omitempty"`
}

// DeviceStatusMap содержит статусы всех устройств
type DeviceStatusMap map[string]DeviceStatus

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "Управление устройствами LoRaWAN",
	Long:  `Команды для создания, удаления и симуляции устройств LoRaWAN из CSV файла`,
}

var createDevicesCmd = &cobra.Command{
	Use:   "create",
	Short: "Создать устройства из CSV файла",
	Long:  `Создает устройства в ChirpStack из указанного CSV файла`,
	RunE:  createDevices,
}

var deleteDevicesCmd = &cobra.Command{
	Use:   "delete",
	Short: "Удалить устройства из списка",
	Long:  `Удаляет устройства из ChirpStack согласно CSV файлу`,
	RunE:  deleteDevices,
}

var simulateDevicesCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Запустить симуляцию устройств",
	Long:  `Запускает симуляцию активных устройств из CSV файла`,
	RunE:  simulateDevices,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Показать статус устройств",
	Long:  `Показывает статус всех устройств из файла состояния`,
	RunE:  showDeviceStatus,
}

func init() {
	// Добавляем флаги для файлов
	devicesCmd.PersistentFlags().StringVarP(&deviceFile, "devices", "d", "devices.csv", "путь к CSV файлу с устройствами")
	devicesCmd.PersistentFlags().StringVarP(&statusFile, "status", "s", "device_status.json", "путь к файлу статуса устройств")

	// Добавляем подкоманды
	devicesCmd.AddCommand(createDevicesCmd)
	devicesCmd.AddCommand(deleteDevicesCmd)
	devicesCmd.AddCommand(simulateDevicesCmd)
	devicesCmd.AddCommand(statusCmd)

	// Добавляем команду devices к root
	rootCmd.AddCommand(devicesCmd)
}

// loadDeviceStatus загружает статус устройств из файла
func loadDeviceStatus(filePath string) (DeviceStatusMap, error) {
	statusMap := make(DeviceStatusMap)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Info("Файл статуса не найден, создается новый")
		return statusMap, nil
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла статуса: %v", err)
	}

	if len(data) == 0 {
		return statusMap, nil
	}

	if err := json.Unmarshal(data, &statusMap); err != nil {
		return nil, fmt.Errorf("ошибка парсинга файла статуса: %v", err)
	}

	return statusMap, nil
}

// saveDeviceStatus сохраняет статус устройств в файл
func saveDeviceStatus(filePath string, statusMap DeviceStatusMap) error {
	data, err := json.MarshalIndent(statusMap, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации статуса: %v", err)
	}

	// Создаем директорию если не существует
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("ошибка создания директории: %v", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("ошибка записи файла статуса: %v", err)
	}

	return nil
}

// readDevicesFromCSVWithStatus читает устройства из CSV с учетом статуса активности
func readDevicesFromCSVWithStatus(filePath string) ([]simulator.DeviceWithStatus, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла %s: %v", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения CSV файла: %v", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV файл пустой")
	}

	var devices []simulator.DeviceWithStatus

	for i, record := range records {
		if i == 0 {
			continue // пропускаем заголовок
		}

		if len(record) == 0 || (len(record) == 1 && record[0] == "") {
			continue // пропускаем пустые строки
		}

		if len(record) < 4 {
			return nil, fmt.Errorf("недостаточно полей в строке %d: ожидается минимум 4 поля, получено %d", i+1, len(record))
		}

		// Валидируем hex-строки
		devEUI := record[2]
		if err := simulator.ValidateHexString(devEUI, 16, "DevEUI"); err != nil {
			return nil, fmt.Errorf("строка %d: %v", i+1, err)
		}

		appKey := record[3]
		if err := simulator.ValidateHexString(appKey, 32, "AppKey"); err != nil {
			return nil, fmt.Errorf("строка %d: %v", i+1, err)
		}

		joinEUI := ""
		if len(record) > 4 && record[4] != "" {
			joinEUI = record[4]
			if err := simulator.ValidateHexString(joinEUI, 16, "JoinEUI"); err != nil {
				return nil, fmt.Errorf("строка %d: %v", i+1, err)
			}
		}

		description := ""
		if len(record) > 5 {
			description = record[5]
		}

		// Проверяем статус активности (7-я колонка, если есть)
		active := true // по умолчанию активно
		if len(record) > 6 && record[6] != "" {
			switch record[6] {
			case "true", "1", "active", "да", "активно":
				active = true
			case "false", "0", "inactive", "нет", "неактивно":
				active = false
			}
		}

		device := simulator.DeviceWithStatus{
			Device: simulator.Device{
				Name:            record[0],
				DeviceProfileId: record[1],
				DevEui:          devEUI,
				NwkKey:          appKey,
				JoinEui:         joinEUI,
				Description:     description,
			},
			Active: active,
		}

		devices = append(devices, device)
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("не найдено валидных устройств в CSV файле")
	}

	return devices, nil
}

func createDevices(cmd *cobra.Command, args []string) error {
	log.Info("Создание устройств из CSV файла...")

	// Инициализируем конфигурацию
	if err := setupASAPIClient(context.Background(), &sync.WaitGroup{}); err != nil {
		return errors.Wrap(err, "ошибка настройки AS API клиента")
	}

	devices, err := readDevicesFromCSVWithStatus(deviceFile)
	if err != nil {
		return err
	}

	// Загружаем текущий статус
	statusMap, err := loadDeviceStatus(statusFile)
	if err != nil {
		return err
	}

	log.Infof("Создание %d устройств...", len(devices))

	var wg sync.WaitGroup
	for _, device := range devices {
		wg.Add(1)
		go func(dev simulator.DeviceWithStatus) {
			defer wg.Done()

			if err := simulator.CreateSingleDevice(dev.Device); err != nil {
				log.WithError(err).WithField("dev_eui", dev.DevEui).Error("ошибка создания устройства")
				return
			}

			// Обновляем статус
			statusMap[dev.DevEui] = DeviceStatus{
				DevEUI: dev.DevEui,
				Name:   dev.Name,
				Active: dev.Active,
			}

			log.WithFields(log.Fields{
				"dev_eui": dev.DevEui,
				"name":    dev.Name,
				"active":  dev.Active,
			}).Info("устройство создано")
		}(device)
	}

	wg.Wait()

	// Сохраняем статус
	if err := saveDeviceStatus(statusFile, statusMap); err != nil {
		return err
	}

	log.Info("Создание устройств завершено")
	return nil
}

func deleteDevices(cmd *cobra.Command, args []string) error {
	log.Info("Удаление устройств из списка...")

	if err := setupASAPIClient(context.Background(), &sync.WaitGroup{}); err != nil {
		return errors.Wrap(err, "ошибка настройки AS API клиента")
	}

	devices, err := readDevicesFromCSVWithStatus(deviceFile)
	if err != nil {
		return err
	}

	statusMap, err := loadDeviceStatus(statusFile)
	if err != nil {
		return err
	}

	log.Infof("Удаление %d устройств...", len(devices))

	var wg sync.WaitGroup
	for _, device := range devices {
		wg.Add(1)
		go func(dev simulator.DeviceWithStatus) {
			defer wg.Done()

			if err := simulator.DeleteSingleDevice(dev.DevEui); err != nil {
				log.WithError(err).WithField("dev_eui", dev.DevEui).Error("ошибка удаления устройства")
				return
			}

			// Удаляем из статуса
			delete(statusMap, dev.DevEui)

			log.WithFields(log.Fields{
				"dev_eui": dev.DevEui,
				"name":    dev.Name,
			}).Info("устройство удалено")
		}(device)
	}

	wg.Wait()

	// Сохраняем обновленный статус
	if err := saveDeviceStatus(statusFile, statusMap); err != nil {
		return err
	}

	log.Info("Удаление устройств завершено")
	return nil
}

func simulateDevices(cmd *cobra.Command, args []string) error {
	log.Info("Запуск симуляции устройств...")

	// Инициализируем все необходимые сервисы
	tasks := []func(context.Context, *sync.WaitGroup) error{
		setLogLevel,
		printStartMessage,
		setupASAPIClient,
		setupASIntegration,
		setupNSIntegration,
		setupPrometheus,
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	for _, t := range tasks {
		if err := t(ctx, &wg); err != nil {
			return err
		}
	}

	// Загружаем устройства
	devices, err := readDevicesFromCSVWithStatus(deviceFile)
	if err != nil {
		return err
	}

	// Фильтруем только активные устройства
	var activeDevices []simulator.DeviceWithStatus
	for _, device := range devices {
		if device.Active {
			activeDevices = append(activeDevices, device)
		}
	}

	log.Infof("Найдено %d активных устройств из %d общих", len(activeDevices), len(devices))

	if len(activeDevices) == 0 {
		log.Warn("Нет активных устройств для симуляции")
		return nil
	}

	// Запускаем симуляцию с активными устройствами
	if err := simulator.StartWithDevices(ctx, &wg, config.C, activeDevices); err != nil {
		return errors.Wrap(err, "ошибка запуска симуляции")
	}

	// Ожидаем сигнала завершения
	exitChan := make(chan struct{})
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	go func() {
		cancel()
		wg.Wait()
		exitChan <- struct{}{}
	}()

	select {
	case <-exitChan:
	case s := <-sigChan:
		log.WithField("signal", s).Info("получен сигнал, завершение симуляции")
	}

	return nil
}

func showDeviceStatus(cmd *cobra.Command, args []string) error {
	statusMap, err := loadDeviceStatus(statusFile)
	if err != nil {
		return err
	}

	if len(statusMap) == 0 {
		fmt.Println("Статус устройств пуст")
		return nil
	}

	fmt.Printf("Статус устройств (%d устройств):\n\n", len(statusMap))
	fmt.Printf("%-20s %-30s %-10s %-20s\n", "DevEUI", "Имя", "Активно", "Последний раз")
	fmt.Printf("%s\n", "--------------------------------------------------------------------------------")

	for _, status := range statusMap {
		activeStr := "Нет"
		if status.Active {
			activeStr = "Да"
		}

		lastSeen := status.LastSeen
		if lastSeen == "" {
			lastSeen = "Никогда"
		}

		fmt.Printf("%-20s %-30s %-10s %-20s\n",
			status.DevEUI,
			status.Name,
			activeStr,
			lastSeen)
	}

	return nil
}
