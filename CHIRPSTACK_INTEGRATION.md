# Интеграция с ChirpStack

Краткая инструкция по настройке JavaScript codec для ChirpStack Simulator.

## 🚀 Быстрая настройка

### 1. Загрузка codec.js

1. Откройте ChirpStack Application Server
2. Перейдите в **Applications** → выберите ваше приложение
3. В разделе **Codec** выберите **JavaScript**
4. Скопируйте содержимое `codec.js` в поле **Script**
5. Нажмите **Save**

### 2. Проверка работы

После настройки codec:
- Устройства будут отправлять payload размером 25 байт
- ChirpStack автоматически декодирует payload в JSON
- В логах приложения вы увидите декодированные данные

## 📊 Пример декодированного payload

**Входные данные (25 байт):**
```
0x01 0x34 0x4E 0x4E 0x45 0x41 0x75 0x74 0x68 0x65 0x6E 0x74 0x69 0x63 0x41 0x4D 0x44 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
```

**Результат декодирования:**
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
    "decoded_at": "2025-08-14T11:20:04.572Z"
  }
}
```

## 🔧 Настройка Application Integration

### HTTP Integration
```json
{
  "headers": {
    "Content-Type": "application/json"
  },
  "url": "http://your-server.com/lorawan-data"
}
```

### MQTT Integration
```json
{
  "topic": "lorawan/uplink/{devEUI}",
  "qos": 1
}
```

## 📝 Структура данных

Каждое uplink сообщение содержит:

| Поле | Описание | Пример |
|------|----------|---------|
| `processor_id` | ID процессора (0-255) | `1` |
| `cpu_mhz` | Частота CPU в МГц | `3300.888` |
| `vendor_id` | Производитель CPU | `"AuthenticAMD"` |
| `status` | Статус данных | `"real_values"` (реальные CPU данные) или `"default_values"` (vendor_id = "unknown") |

## ⚠️ Возможные проблемы

### 1. Codec не работает
- Проверьте синтаксис JavaScript в ChirpStack
- Убедитесь, что функция `decodeUplink` определена
- Проверьте логи ChirpStack на ошибки

### 2. Неправильный размер payload
- Payload должен быть ровно 25 байт
- Проверьте настройки симулятора

### 3. Ошибки декодирования
- Убедитесь, что формат данных соответствует ожидаемому
- Проверьте порядок байт (little-endian для CPU MHz)

## 🧪 Тестирование

Для тестирования codec используйте:
```bash
node test_codec.js
```

## 📚 Дополнительная документация

- `CODEC_README.md` - Подробное описание codec
- `CSV_DEVICES_README.md` - Настройка устройств
- `README.md` - Основная документация проекта
