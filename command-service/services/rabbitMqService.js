
const amqp = require('amqplib');

let channel = null;
let connection = null;

const connectRabbitMQ = async () => {
    const MAX_RETRIES = 10;
    const RETRY_DELAY = 5000; // 5 seconds

    for (let i = 0; i < MAX_RETRIES; i++) {
        try {
            // Connect to the RabbitMQ server
            const rabbitMqUrl = process.env.RABBITMQ_URL || 'amqp://guest:guest@rabbitmq:5672/';
            console.log(`Attempting to connect to RabbitMQ(Attempt ${i + 1}/${MAX_RETRIES})...`);
            connection = await amqp.connect(rabbitMqUrl);

            // Create a channel
            channel = await connection.createChannel();

            console.log("✅ RabbitMQ Connected and Channel Created");

            // Handle connection close/error events
            connection.on('error', (err) => {
                console.error('RabbitMQ connection error', err);
            });
            return; // Successfully connected

        } catch (error) {
            console.error(`Failed to connect to RabbitMQ(Attempt ${i + 1}/${MAX_RETRIES}): `, error.message);
            if (i === MAX_RETRIES - 1) {
                console.error("Max retries reached. Exiting...");
                process.exit(1);
            }
            await new Promise(resolve => setTimeout(resolve, RETRY_DELAY));
        }
    }
};

const publishToQueue = async (queueName, data) => {
    if (!channel) {
        console.error("RabbitMQ channel not initialized. Call connectRabbitMQ first.");
        return;
    }

    try {
        // Ensure queue exists (idempotent)
        await channel.assertQueue(queueName, { durable: true });

        // Convert object to Buffer and send
        const messageBuffer = Buffer.from(JSON.stringify(data));
        channel.sendToQueue(queueName, messageBuffer);

        console.log(`[x] Sent to ${queueName}: `, data);
    } catch (error) {
        console.error("Error publishing message:", error);
    }
};

module.exports = {
    connectRabbitMQ,
    publishToQueue
};