const express = require('express')
const db = require('./db')
const app = express()
const fs = require('fs');
const path = require('path');
const api = require('./routes/ticket')
const { connectRabbitMQ } = require('./services/rabbitMqService')
const swaggerUi = require('swagger-ui-express');
const swaggerSpecs = require('./swagger');

app.get('/initiate', async (req, res) => {
  try {
    const scriptsDir = path.join(__dirname, '..', 'docker', 'postgres', 'scripts');
    const scripts = ['01-schema.sql', '02-seed-data.sql', '03-indexes.sql'];

    console.log('Starting database initialization...');

    for (const script of scripts) {
      const sqlPath = path.join(scriptsDir, script);
      if (fs.existsSync(sqlPath)) {
        const sql = fs.readFileSync(sqlPath, 'utf8');
        console.log(`Running Script: ${script}`);
        await db.query(sql);
      } else {
        console.warn(`Script not found: ${script}`);
      }
    }

    console.log('Database initialization completed successfully.');
    res.send('Database initialized successfully');
  } catch (err) {
    console.error('Error during database initialization:', err);
    res.status(500).send('Internal Server Error: ' + err.message);
  }
});

app.get('/health', (req, res) => {
  res.status(200).json({ status: 'UP', timestamp: new Date() });
});

app.use(express.urlencoded({ extended: false })); //built in middlware to handlw urlencoded data (form data)
app.use('/api-docs', swaggerUi.serve, swaggerUi.setup(swaggerSpecs));
app.use(express.json());

app.use('/api/tickets', api);

// Initial Checks
db.testConnection().then(() => {
  connectRabbitMQ().then(() => {
    app.listen(5000, () => {
      console.log("CONNECTED TO RABBITMQ & SERVER RUNNING ON PORT 5000")
    })
  });
});
