# Как обновлять кофигурацию балансировщика

Так как все балансировщики умеют читать из файла, то нужно уметь обновлять файл.<br>
В k8s используется для конфигурации configmap, но в nomad такой сущности нет.<br>

# Спецификация контроллера

```
IngressPluginConfig {
    consul_endpoint = "https://my-consul:8501"
    consul_token = ""
    nomad_endpoint = "https://my-nomad:4646"
    nomad_token = ""
    cert_auth_file_path = "/certs/cert.crt"
    ingress_class = ""
    enable_metrics = "true/false"
    internal {
        volume_name = ""
    }
    external_name {
    
    }
}
```

# Спецификация Ingress в Job

Из-за отсутствия CRD, которые есть в k8s, нужно будет сделать общий вид, основываясь на тегах, которые есть в Jobs.

# Требования к ingress-plugin



# Требования к Ingress-plugin

1. HealthCheck контроллера
2. Создание правил роутинга
3. Изменение правил роутинга
4. Удаление правил роутинга

# Принцип работы Ingress-controller

1. Ingress-plugin запрашивает все job
2. Для каждого job извлекаются тэги
3. Если нужных тегов нет, то джоба пропускается
4. По полученным job запрашивается список allocation
5. Запрашивается информация об allocation в консуле, чтобы посмотреть healthchecks
6. Если allocation здоров, то запрашивается информация по нему, чтобы найти ip:port, куда роутить трафик
7. На основании тегов строятся правила роутинга на ip:port
8. Полученная конфигурация записывается в файл на ноде, где располагается балансировщик

# Как запустить кластер с Ingress и Ingress controller