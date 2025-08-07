# Расширенное управление устройствами ChirpStack Simulator

## Описание

В проект ChirpStack Simulator добавлен расширенный функционал управления устройствами LoRaWAN через командную строку. Теперь вы можете:

- ✅ Создавать устройства из CSV файла
- ✅ Удалять устройства по списку из CSV
- ✅ Запускать симуляцию только активных устройств
- ✅ Сохранять статус устройств между запусками
- ✅ Управлять активностью устройств через CSV файл

## Формат CSV файла

CSV файл должен использовать точку с запятой (`;`) как разделитель полей и содержать следующие колонки:

```
Name (string);Profile;DevEUI (hex string);AppKey (hex string);Join EUI (опционально) (hex string);Description (string);Active (true/false)
```

### Описание полей:

1. **Name** (обязательное) - имя устройства
2. **Profile** (обязательное) - ID профиля устройства (UUID)
3. **DevEUI** (обязательное) - уникальный идентификатор устройства (16 hex символов)
4. **AppKey** (обязательное) - ключ приложения (32 hex символа)
5. **JoinEUI** (опциональное) - идентификатор присоединения (32 hex символа)
6. **Description** (опциональное) - описание устройства
7. **Active** (опциональное) - статус активности устройства (true/false, по умолчанию true)

### Пример CSV файла:

```csv
Name (string);Profile;DevEUI (hex string);AppKey (hex string);Join EUI (опционально) (hex string);Description (string);Active (true/false)
lorawan-emulator-01;98e37811-de41-4da7-9440-f3c8fb35fbb9;7700000000000001;2b7e151628aed2a6abf7158809cf4f3c;10000000000000000000000000000000;LoRaWAN сенсор 1;true
lorawan-emulator-02;98e37811-de41-4da7-9440-f3c8fb35fbb9;7700000000000002;2b7e151628aed2a6abf7158809cf4f3c;10000000000000000000000000000000;LoRaWAN сенсор 2;false
```

## Валидация данных

Функция автоматически проверяет:

- ✅ Наличие обязательных полей (Name, Profile, DevEUI, AppKey)
- ✅ Корректность hex-строк для DevEUI (16 символов)
- ✅ Корректность hex-строк для AppKey (32 символа)  
- ✅ Корректность hex-строк для JoinEUI (32 символа, если указан)
- ✅ Валидация статуса активности (поддерживает: true/false, 1/0, active/inactive, да/нет)
- ✅ Пропуск пустых строк
- ✅ Детальные сообщения об ошибках с указанием номера строки

## Использование

### Команды управления устройствами

Все команды поддерживают флаги для указания файлов:
- `-d, --devices` - путь к CSV файлу с устройствами (по умолчанию: devices.csv)
- `-s, --status` - путь к файлу статуса устройств (по умолчанию: device_status.json)

#### 1. Создание устройств
```bash
# Создать устройства из файла devices.csv
./chirpstack-simulator devices create

# Использовать другой CSV файл
./chirpstack-simulator devices create -d my_devices.csv
```

#### 2. Удаление устройств
```bash
# Удалить устройства согласно списку в devices.csv
./chirpstack-simulator devices delete

# Использовать другой CSV файл
./chirpstack-simulator devices delete -d devices_to_remove.csv
```

#### 3. Запуск симуляции
```bash
# Запустить симуляцию только активных устройств
./chirpstack-simulator devices simulate

# Использовать другой CSV файл
./chirpstack-simulator devices simulate -d active_devices.csv
```

#### 4. Просмотр статуса устройств
```bash
# Показать статус всех устройств
./chirpstack-simulator devices status

# Использовать другой файл статуса
./chirpstack-simulator devices status -s my_status.json
```

### Традиционное использование

1. Создайте CSV файл с именем `devices.csv` в корневой директории проекта
2. Заполните его данными устройств в соответствии с форматом выше
3. Используйте команды CLI для управления устройствами

## Структура данных

Устройства представлены следующей структурой:

```go
type Device struct {
    Name            string  // Имя устройства
    DeviceProfileId string  // ID профиля устройства
    DevEui          string  // DevEUI (hex)
    NwkKey          string  // AppKey (hex) 
    JoinEui         string  // JoinEUI (hex, опционально)
    Description     string  // Описание (опционально)
}

type DeviceWithStatus struct {
    Device
    Active bool    // Статус активности
}

type DeviceStatus struct {
    DevEUI   string `json:"dev_eui"`
    Name     string `json:"name"`
    Active   bool   `json:"active"`
    LastSeen string `json:"last_seen,omitempty"`
}
```

## Сохранение статуса устройств

Симулятор автоматически сохраняет статус устройств в JSON файл (`device_status.json` по умолчанию). Этот файл содержит:

- DevEUI устройства
- Имя устройства
- Статус активности (активно/неактивно)
- Время последней активности (в будущих версиях)

### Пример файла статуса:

```json
{
  "7700000000000001": {
    "dev_eui": "7700000000000001",
    "name": "lorawan-emulator-01",
    "active": true,
    "last_seen": ""
  },
  "7700000000000002": {
    "dev_eui": "7700000000000002", 
    "name": "lorawan-emulator-02",
    "active": false,
    "last_seen": ""
  }
}
```

Статус сохраняется при:
- Создании устройств
- Удалении устройств
- Изменении активности через CSV

## Обработка ошибок

Функция возвращает подробные ошибки в случае:
- Невозможности открыть файл
- Некорректного формата CSV
- Недостающих обязательных полей
- Неверного формата hex-строк
- Пустого файла или отсутствия валидных устройств

## Логирование

При загрузке устройств из CSV выводятся информационные сообщения:
- Информация о каждом обрабатываемом устройстве
- Успешная инициализация устройства с DevEUI и AppKey
- Ошибки валидации с указанием конкретного поля

## Примеры использования

### Типичный рабочий процесс:

1. **Подготовка устройств:**
```bash
# Создать устройства из CSV файла
./chirpstack-simulator devices create -d devices.csv
```

2. **Проверка статуса:**
```bash
# Посмотреть какие устройства созданы
./chirpstack-simulator devices status
```

3. **Запуск симуляции:**
```bash
# Запустить симуляцию только активных устройств
./chirpstack-simulator devices simulate -d devices.csv
```

4. **Управление устройствами:**
```bash
# Изменить статус устройств в CSV файле (поменять active на false)
# Запустить симуляцию с новыми настройками
./chirpstack-simulator devices simulate -d devices.csv

# Удалить ненужные устройства
./chirpstack-simulator devices delete -d devices_to_remove.csv
```

### Работа с разными наборами устройств:

```bash
# Тестовые устройства
./chirpstack-simulator devices create -d test_devices.csv -s test_status.json
./chirpstack-simulator devices simulate -d test_devices.csv -s test_status.json

# Продуктивные устройства
./chirpstack-simulator devices create -d prod_devices.csv -s prod_status.json
./chirpstack-simulator devices simulate -d prod_devices.csv -s prod_status.json
```

## Совместимость

- ✅ Полностью совместим с существующим кодом симулятора
- ✅ Не влияет на другие части системы
- ✅ Обратная совместимость с CSV файлами без колонки Active
- ✅ Поддержка существующих конфигурационных файлов