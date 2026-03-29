package cpuinfo

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// CPUInfo представляет информацию о процессоре
type CPUInfo struct {
	Processor uint8   `json:"processor"`
	CPUMHz    float32 `json:"cpu_mhz"`
	VendorID  string  `json:"vendor_id"`
}

// GetCPUInfo читает информацию о процессоре из /proc/cpuinfo
func GetCPUInfo(processorID uint8) (*CPUInfo, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc/cpuinfo: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var currentProcessor uint8
	var cpuMHz float32
	var vendorID string
	var foundProcessor bool

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "processor") {
			// Извлекаем номер процессора из строки "processor       : 0"
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				if procID, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 8); err == nil {
					currentProcessor = uint8(procID)
					// Сбрасываем значения для нового процессора
					cpuMHz = 0
					vendorID = ""

					// Если это нужный нам процессор, отмечаем что нашли
					if currentProcessor == processorID {
						foundProcessor = true
					}
				}
			}
		} else if strings.HasPrefix(line, "cpu MHz") && foundProcessor {
			// Извлекаем частоту CPU для найденного процессора
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				freqStr := strings.TrimSpace(parts[1])
				if freq, err := strconv.ParseFloat(freqStr, 32); err == nil {
					cpuMHz = float32(freq)
				}
			}
		} else if strings.HasPrefix(line, "vendor_id") && foundProcessor {
			// Извлекаем vendor ID для найденного процессора
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				vendorID = strings.TrimSpace(parts[1])
			}
		}

		// Если собрали всю информацию для нужного процессора, выходим
		if foundProcessor && cpuMHz > 0 && vendorID != "" {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading /proc/cpuinfo: %w", err)
	}

	// Если не нашли информацию для нужного процессора, возвращаем значения по умолчанию
	if !foundProcessor || cpuMHz == 0 || vendorID == "" {
		return &CPUInfo{
			Processor: processorID,
			CPUMHz:    0.0,
			VendorID:  "unknown",
		}, nil
	}

	return &CPUInfo{
		Processor: currentProcessor,
		CPUMHz:    cpuMHz,
		VendorID:  vendorID,
	}, nil
}

// GetDefaultCPUInfo возвращает значения по умолчанию
func GetDefaultCPUInfo() *CPUInfo {
	return &CPUInfo{
		Processor: 0,
		CPUMHz:    0.0,
		VendorID:  "unknown",
	}
}

// ToPayload конвертирует CPUInfo в байтовый массив
func (c *CPUInfo) ToPayload() []byte {
	// Размер payload: 1 (processor) + 4 (cpu_mhz) + 20 (vendor_id) = 25 байт
	payload := make([]byte, 25)

	// Processor ID (1 байт)
	payload[0] = c.Processor

	// CPU MHz (4 байта, little-endian) - используем битовое представление float32
	binary.LittleEndian.PutUint32(payload[1:5], uint32(math.Float32bits(c.CPUMHz)))

	// Vendor ID (20 байт, обрезаем или дополняем нулями)
	vendorBytes := []byte(c.VendorID)
	if len(vendorBytes) > 20 {
		copy(payload[5:25], vendorBytes[:20])
	} else {
		copy(payload[5:5+len(vendorBytes)], vendorBytes)
		// Остальные байты уже заполнены нулями
	}

	return payload
}

// FromPayload восстанавливает CPUInfo из байтового массива
func FromPayload(payload []byte) (*CPUInfo, error) {
	if len(payload) < 25 {
		return nil, fmt.Errorf("payload too short: expected 25 bytes, got %d", len(payload))
	}

	processor := payload[0]
	// CPU MHz (4 байта, little-endian) - восстанавливаем из битового представления float32
	cpuMHz := math.Float32frombits(binary.LittleEndian.Uint32(payload[1:5]))

	// Извлекаем vendor_id, убирая нулевые байты
	vendorBytes := payload[5:25]
	vendorID := strings.TrimRight(string(vendorBytes), "\x00")

	return &CPUInfo{
		Processor: processor,
		CPUMHz:    cpuMHz,
		VendorID:  vendorID,
	}, nil
}
