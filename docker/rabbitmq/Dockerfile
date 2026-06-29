# RabbitMQ для HealthPage с плагином отложенных сообщений (DESIGN §8.1).
# Базовый образ официальный; поверх — community-плагин rabbitmq_delayed_message_exchange,
# который даёт exchange типа x-delayed-message (отложенные анонсы работ + retry-with-backoff).
# Версия плагина должна совпадать с минорной версией RabbitMQ (здесь — 3.13.x).
FROM rabbitmq:3.13-management-alpine

ARG PLUGIN_VERSION=3.13.0
ADD --chmod=644 \
    https://github.com/rabbitmq/rabbitmq-delayed-message-exchange/releases/download/v${PLUGIN_VERSION}/rabbitmq_delayed_message_exchange-${PLUGIN_VERSION}.ez \
    /opt/rabbitmq/plugins/

# Включаем плагин в образе (offline — без запущенного брокера).
RUN rabbitmq-plugins enable --offline rabbitmq_delayed_message_exchange
