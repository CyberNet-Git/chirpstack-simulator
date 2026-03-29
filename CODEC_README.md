# ChirpStack Simulator Codec

Файл `codec.js` содержит JavaScript декодер для payload от LoRaWAN устройств симулятора ChirpStack.

## Описание

Декодер преобразует закодированные байты payload в читаемый JSON формат, извлекая информацию о процессоре системы.

## Структура Payload

Каждый payload имеет размер **25 байт** и содержит:

| Поле | Размер | Описание | Пример |
|------|--------|----------|---------|
| **Processor ID** | 1 байт | ID процессора (0-255) | `0x01` = процессор 1 |
| **CPU MHz** | 4 байта | Частота процессора (float32, little-endian) | `3310.996 МГц` |
| **Vendor ID** | 20 байт | Производитель процессора (строка + padding) | `"AuthenticAMD"` |

## Использование в ChirpStack

### 1. Загрузка в ChirpStack

1. Откройте ChirpStack Application Server
2. Перейдите в раздел **Applications** → **Codec**
3. Загрузите файл `codec.js`
4. Убедитесь, что функция `decodeUplink` доступна

### 2. Настройка Application Integration

В настройках интеграции приложения укажите:
- **Codec**: JavaScript
- **Script**: Содержимое `codec.js`

## Функции

### `decodeUplink(input)`

Основная функция декодирования.

**Параметры:**
- `input.fPort` - Порт приложения
- `input.bytes` - Массив байт payload

**Возвращает:**
```json
{
  "data": {
    "processor_info": {
      "processor_id": 1,
      "cpu_mhz": 3300.888,
      "vendor_id": "AuthenticAMD",
      "status": "real_values",
      "description": "Device using real CPU info from system"
    }
  },
  "metadata": {
    "payload_size": 25,
    "f_port": 1,
    "decoded_at": "2025-08-14T11:18:18.180Z"
  }
}
```

### `testDecoder(testBytes)`

Функция для тестирования декодера.

## Примеры Payload

### Пример 1: Real CPU Info (Processor ID = 1)
```javascript
var payload = [
    0x01,           // Processor ID = 1
    0x34, 0x4E, 0x4E, 0x45,  // CPU MHz = 3310.996
    0x41, 0x75, 0x74, 0x68, 0x65, 0x6E, 0x74, 0x69, 0x63, 0x41, 0x4D, 0x44, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00  // "AuthenticAMD"
];
```

**Результат:**
```json
{
  "data": {
    "processor_info": {
      "processor_id": 1,
      "cpu_mhz": 3300.888,
      "vendor_id": "AuthenticAMD",
      "status": "real_values"
    }
  }
}
```

### Пример 2: Default Values (Processor ID = 0)
```javascript
var payload = [
    0x00,           // Processor ID = 0
    0x00, 0x00, 0x00, 0x00,  // CPU MHz = 0.0
    0x75, 0x6E, 0x6B, 0x6E, 0x6F, 0x77, 0x6E, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00  // "unknown"
];
```

**Результат:**
```json
{
  "data": {
    "processor_info": {
      "processor_id": 0,
      "cpu_mhz": 0,
      "vendor_id": "unknown",
      "status": "default_values"
    }
  }
}
```

## Статусы

Декодер автоматически определяет статус данных:

- **`real_values`** - Устройство использует реальные данные CPU из системы (любой processor_id с валидным vendor_id)
- **`default_values`** - Устройство использует значения по умолчанию (vendor_id = "unknown")

**Важно:** Processor ID = 0 является **реальным процессором** в системе и будет иметь статус `real_values`, если vendor_id не равен "unknown".

## Тестирование

Для тестирования декодера используйте Node.js:

```bash
node -e "
const codec = require('./codec.js');
const testBytes = [0x01, 0x34, 0x4E, 0x4E, 0x45, ...];
console.log(JSON.stringify(codec.testDecoder(testBytes), null, 2));
"
```

## Совместимость

- **ChirpStack**: Полностью совместим
- **Node.js**: Поддерживается через `require()`
- **Браузер**: Доступен через `window.decodeUplink`

## Обработка ошибок

Декодер проверяет:
- Размер payload (должен быть 25 байт)
- Корректность данных
- Возвращает понятные сообщения об ошибках

## Интеграция с симулятором

Этот декодер специально разработан для работы с payload от `chirpstack-simulator`, который:

1. Читает информацию о процессорах из `/proc/cpuinfo`
2. Формирует payload размером 25 байт
3. Отправляет данные через LoRaWAN uplink

## Поддержка

При возникновении проблем проверьте:
- Размер payload (25 байт)
- Формат CPU MHz (4 байта, little-endian)
- Корректность Vendor ID (20 байт)
- Логи ChirpStack на наличие ошибок JavaScript
