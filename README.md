# zosma-sd-server


### Detailed Overview of a Task Queue Manager for a Distributed Stable Diffusion Worker Network

The Task Queue Manager described acts as a central hub in a distributed network architecture that leverages asynq and Redis to manage tasks for a network of workers dedicated to running Stable Diffusion image generation tasks. This system is designed to handle large volumes of image generation requests efficiently, ensuring that tasks are processed quickly and reliably across various worker nodes.

#### **Components of the System**

- **Task Queue Manager**: Utilizes `asynq`, a simple and efficient asynchronous task queue and scheduler written in Go. `Asynq` primarily interfaces with Redis, which acts as the task storage and management backend. This setup allows the Task Queue Manager to effectively distribute tasks, manage retries for failed tasks, and schedule tasks for future execution. 

- **Redis**: Serves as a high-performance data structure store that provides a robust, in-memory key-value database to handle queues. It supports various data structures such as strings, hashes, lists, sets, sorted sets with range queries, and streams. Redis is highly optimized for performance and can handle a large volume of tasks, making it ideal for high-load environments.

- **Stable Diffusion Worker**: Each worker in the network is an instance that interfaces with the Stable Diffusion model via a WebUI API. This API integration enables the workers to generate images based on textual prompts received from the Task Queue Manager. The workers are designed to be stateless and scalable, allowing for dynamic scaling based on the workload.

#### **Workflow and Interaction**

1. **Client Application Submission**: Client applications, such as a Discord bot named `zosma-discord-bot`, submit image generation requests to the Task Queue Manager. These applications interact with the Task Queue Manager through a client interface, exemplified by `clients/client.go`, which provides a Go-based API to enqueue tasks.

2. **Task Enqueuing**: Upon receiving a request, the Task Queue Manager uses `asynq` to enqueue a task in Redis. Each task encapsulates all the necessary information for generating an image, including the text prompt and any user-specified parameters.

3. **Task Distribution and Processing**: The Task Queue Manager distributes tasks among available workers based on current load and worker availability. This distribution is managed through Redis, ensuring that tasks are reliably stored and retrieved across the network.

4. **Image Generation**: Workers retrieve tasks from Redis, generate images using the Stable Diffusion WebUI API, and then return the results. The API allows workers to communicate with the core model, process the input prompts, and create the corresponding images.

5. **Result Handling**: Once an image is generated, the result is sent back to the originating client application through the Task Queue Manager, completing the cycle.

#### **Scalability and Reliability**

- **Dynamic Scaling**: The distributed nature of the worker network, combined with the stateless design of each worker, allows for dynamic scaling. Additional workers can be added to the network without significant configuration, enabling the system to handle increased load seamlessly.

- **Fault Tolerance**: The use of Redis and its inherent persistence mechanisms, along with `asynq`'s support for retries and scheduled tasks, enhances the fault tolerance of the system. Tasks that fail due to worker errors or system issues can be retried automatically.


This architecture provides a robust and scalable solution for managing a distributed network of Stable Diffusion workers. By leveraging modern tools like `asynq`, Redis, and the Stable Diffusion WebUI API, the system ensures efficient task management, high performance, and reliability, making it suitable for applications requiring high-volume and high-speed image generation capabilities.


### Visit following repos for additional info:
- [asynq](https://github.com/hibiken/asynq) 
- [redis](https://github.com/redis/redis).

- [Stable Diffusion WebUI API](https://github.com/AUTOMATIC1111/stable-diffusion-webui)

- [zosma-discord-bot](https://github.com/rampenke/zosma-discord-bot) 
## Build
```
docker build -t zosma-sd-webserver -f Dockerfile.webserver .
docker build -t zosma-sd-workershim -f Dockerfile.workershim .
```
## local testing

```
docker run -it --rm --env-file .env -p 1324:1324 zosma-sd-webserver
```