export default () => ({
  port: parseInt(process.env.PORT, 10) || 3000,
  database: {
    uri: process.env.MONGODB_URI || 'mongodb://192.168.11.128:27017/github-trend',
  },
  github: {
    apiToken: process.env.GITHUB_API_TOKEN,
  },
  google: {
    clientId: process.env.GOOGLE_CLIENT_ID,
    clientSecret: process.env.GOOGLE_CLIENT_SECRET,
    callbackUrl: process.env.GOOGLE_CALLBACK_URL,
  },
}); 