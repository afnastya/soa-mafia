# SOA-MAFIA
## Запуск
```
docker-compose build
docker-compose up --scale client=4 -d
```

Команда, чтобы читать логи  сервера:
```
docker logs -f soa-mafia_mafia_server_1
```

В каждом клиенте запущено консольное приложение. Необходимо подключиться к ним, чтобы писать команды:
```
docker logs <container_name> && docker attach <container_name>
```
Или для удобства можно использовать подготовленный скрипт:
```
./attach.sh <n>         # n - номер клиента, число от 1 до 4 включительно
```

После ввода имени игрока появится надпись "Enter a command:". Чтобы получить список поддерживаемых команд, напишите "help".