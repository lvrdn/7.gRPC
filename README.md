# 7.gRPC

Цель задания - создание микросервиса, осуществляющего отправку данных статистики и логирования по потоковому интерфейсу. 
Особенностью микросервиса является возможность подключения двух и более клиентов-получателей статистики и логов.
Применен паттерн fan-out - передача данных о новых событиях осущестляется через каналы, методы-обработчики паралелльно обрабатывают данные и отсылают ответ на сторону клиента.
Обработка новых событий и система контроля доступа (ACL) реализованы в интерсепторах с объединением их в chain.

Перечень основных файлов проекта:
1. service.go - разработанный код программы, включающий систему контроля доступа и обработку новых событий в интерсепторах, методы-обработчики статистики и логов, инициализацию микросервиса.
2. HW_readme.md - описание условий задания.
3. service_test.go - приложенные к условиям задания тесты.
4. service.proto - приложеная к условиям задания съема для генерации grpc-обвязки.
5. service_grpc.pb.go , service.pb.go - сгенерированные файлы.
