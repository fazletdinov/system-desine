// Package userchecker — мост между контекстами: реализует порт chat/usecase.UserChecker
// поверх use case контекста user. Это единственное место, где chat знает про user
// (правило depguard «chat-context» делает для него исключение). В модуле 6 на его
// месте будет gRPC-клиент сервиса пользователей.
package userchecker
