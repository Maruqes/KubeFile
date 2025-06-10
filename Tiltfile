# Allow the Default context (k0s local cluster)
load('ext://dotenv', 'dotenv')
load('ext://restart_process', 'docker_build_with_restart')
dotenv('.env')
allow_k8s_contexts(os.getenv('K8_CONTEXT'))

# Configure Tilt for faster development - increased parallelism and reduced timeouts
update_settings(
    max_parallel_updates=15, 
    k8s_upsert_timeout_secs=30,
    suppress_unused_image_warnings=None
)

# Watch for changes in Go files for faster rebuilds
watch_file('./go.mod')
watch_file('./go.sum')

# Helper function to create optimized live updates
def go_live_update(service_path, binary_name, cgo_enabled="0"):
    return [
        # Sync Go source files (only when changed)
        sync(service_path, '/app/' + service_path),
        sync('./shared/', '/app/shared/'),
        sync('./go.mod', '/app/go.mod'),
        sync('./go.sum', '/app/go.sum'),
        # Fast rebuild with optimized flags and parallel builds
        run(
            'cd /app && CGO_ENABLED={} GOOS=linux go build -ldflags="-s -w" -trimpath -o {} {}'.format(
                cgo_enabled, binary_name, service_path
            ), 
            trigger=[service_path, './shared/']
        ),
    ]

# Build shortener service with live update
docker_build_with_restart(
    'shortener-service', 
    '.', 
    dockerfile='./services/shortener/Dockerfile',
    entrypoint=['./shortener-service'],
    # Only rebuild when these files change
    only=[
        './services/shortener/',
        './shared/',
        './go.mod',
        './go.sum'
    ],
    # Optimized live update
    live_update=go_live_update('./services/shortener/', 'shortener-service', "1"),
    # Faster builds
    cache_from=['shortener-service:latest'],
)

# Build gateway service with live update
docker_build_with_restart(
    'gateway-service', 
    '.', 
    dockerfile='./services/gateway/Dockerfile',
    entrypoint=['./gateway-service'],
    # Only rebuild when these files change
    only=[
        './services/gateway/',
        './shared/',
        './go.mod',
        './go.sum'
    ],
    # Optimized live update  
    live_update=go_live_update('./services/gateway/', 'gateway-service', "0"),
    # Faster builds
    cache_from=['gateway-service:latest'],
)

# Deploy to Kubernetes
k8s_yaml(['k8s/shortener-service.yaml', 'k8s/gateway-service.yaml', 'k8s/redis-statefulset.yaml'])

# Create resources with optimized settings
k8s_resource(
    'redis-master', 
    port_forwards='6379:6379',
    auto_init=True,
    trigger_mode=TRIGGER_MODE_AUTO,
    resource_deps=[],
)


k8s_resource(
    'shortener-service', 
    port_forwards='50051:50051',
    auto_init=True,
    trigger_mode=TRIGGER_MODE_AUTO,
    resource_deps=[],
)

k8s_resource(
    'gateway-service', 
    port_forwards='8080:8080',
    auto_init=True,
    trigger_mode=TRIGGER_MODE_AUTO,
    resource_deps=['shortener-service'],
)