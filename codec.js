/**
 * ChirpStack Simulator Codec
 * Декодирует payload от LoRaWAN устройств симулятора
 * 
 * Структура payload (25 байт):
 * - Processor ID: 1 байт (0-255)
 * - CPU MHz: 4 байта (float32, little-endian)
 * - Vendor ID: 20 байт (строка, дополненная нулями)
 */

// Вспомогательные функции для работы с байтами
function intFromBytesLE(bytes) {
    var value = 0;
    for (var i = 0; i < bytes.length; i++) {
        value += bytes[i] << (8 * i);
    }
    return value;
}

function floatFromBytesLE(bytes) {
    // Преобразуем 4 байта в float32
    var buffer = new ArrayBuffer(4);
    var view = new DataView(buffer);
    
    for (var i = 0; i < 4; i++) {
        view.setUint8(i, bytes[i]);
    }
    
    return view.getFloat32(0, true); // true = little-endian
}

function stringFromBytes(bytes) {
    // Преобразуем байты в строку, убирая нулевые байты
    var result = "";
    for (var i = 0; i < bytes.length; i++) {
        if (bytes[i] === 0) break;
        result += String.fromCharCode(bytes[i]);
    }
    return result;
}

/**
 * Основная функция декодирования uplink
 * @param {Object} input - Входные данные от ChirpStack
 * @param {number} input.fPort - Порт приложения
 * @param {Array} input.bytes - Массив байт payload
 * @returns {Object} Декодированные данные в формате JSON
 */
function decodeUplink(input) {
    // Проверяем, что payload имеет правильный размер (25 байт)
    if (input.bytes.length !== 25) {
        return {
            "error": "Invalid payload length",
            "expected": 25,
            "received": input.bytes.length
        };
    }
    
    // Извлекаем данные из payload
    var processorId = input.bytes[0];
    var cpuMHz = floatFromBytesLE(input.bytes.slice(1, 5));
    var vendorId = stringFromBytes(input.bytes.slice(5, 25));
    
    // Формируем результат
    var result = {
        "data": {
            "processor_info": {
                "processor_id": processorId,
                "cpu_mhz": parseFloat(cpuMHz.toFixed(3)), // Округляем до 3 знаков
                "vendor_id": vendorId
            }
        }
    };
    
    // Добавляем дополнительную информацию в зависимости от processor_id
    if (vendorId === "unknown") {
        result.data.processor_info.status = "default_values";
        result.data.processor_info.description = "Device using default CPU info values";
    } else {
        result.data.processor_info.status = "real_values";
        result.data.processor_info.description = "Device using real CPU info from system";
    }
    
    // Добавляем метаданные
    result.data.metadata = {
        "payload_size": input.bytes.length,
        "f_port": input.fPort,
        "decoded_at": new Date().toISOString()
    };
    
    return result;
}

/**
 * Функция для тестирования декодера
 * @param {Array} testBytes - Тестовые байты для проверки
 * @returns {Object} Результат декодирования
 */
function testDecoder(testBytes) {
    var testInput = {
        fPort: 1,
        bytes: testBytes
    };
    
    return decodeUplink(testInput);
}

// Примеры использования:

// 1. Payload для processor ID = 1, CPU MHz = 3310.996, Vendor ID = "AuthenticAMD"
var example1 = [
    0x01,           // Processor ID = 1
    0x34, 0x4E, 0x4E, 0x45,  // CPU MHz = 3310.996 (little-endian float32)
    0x41, 0x75, 0x74, 0x68, 0x65, 0x6E, 0x74, 0x69, 0x63, 0x41, 0x4D, 0x44, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00  // "AuthenticAMD" + padding
];

// 2. Payload для processor ID = 0, CPU MHz = 0.0, Vendor ID = "unknown"
var example2 = [
    0x00,           // Processor ID = 0
    0x00, 0x00, 0x00, 0x00,  // CPU MHz = 0.0
    0x75, 0x6E, 0x6B, 0x6E, 0x6F, 0x77, 0x6E, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00  // "unknown" + padding
];

// 3. Payload для processor ID = 5, CPU MHz = 2400.0, Vendor ID = "Intel"
var example3 = [
    0x05,           // Processor ID = 5
    0x00, 0x20, 0x16, 0x45,  // CPU MHz = 2400.0 (little-endian float32)
    0x49, 0x6E, 0x74, 0x65, 0x6C, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00  // "Intel" + padding
];

// Экспортируем функции для использования в ChirpStack
if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
        decodeUplink: decodeUplink,
        testDecoder: testDecoder
    };
}

// Для использования в браузере или ChirpStack
if (typeof window !== 'undefined') {
    window.decodeUplink = decodeUplink;
    window.testDecoder = testDecoder;
}
