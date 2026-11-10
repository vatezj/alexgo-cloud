import { createApp } from './app.js';

const port = process.env.PORT || 80;
createApp().listen(port, () => console.log(`zhiwei service running on port ${port}`));
