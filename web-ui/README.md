# LaForge Web UI

A modern, responsive web interface for LaForge built with Preact and TypeScript.

## Features

- 🚀 **Preact** - Fast, lightweight React alternative
- 🔧 **TypeScript** - Type-safe development
- 📦 **Vite** - Lightning-fast build tool
- 🎨 **Tailwind CSS** - Utility-first CSS framework
- 🔌 **WebSocket** - Real-time updates
- 📱 **Responsive** - Mobile-first design
- 🧪 **Vitest** - Unit testing framework
- 📏 **ESLint + Prettier** - Code quality and formatting

## Development

### Prerequisites

- Node.js 18+ 
- npm or yarn

### Setup

1. Install dependencies:
   ```bash
   npm install
   ```

2. Copy environment variables:
   ```bash
   cp .env.example .env.local
   ```

3. Start development server:
   ```bash
   npm run dev
   ```

The application will be available at `http://localhost:3000`.

### Available Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint
- `npm run lint:fix` - Fix ESLint issues
- `npm run format` - Format code with Prettier
- `npm run format:check` - Check code formatting
- `npm run test` - Run tests
- `npm run test:ui` - Run tests with UI
- `npm run test:coverage` - Run tests with coverage

## Project Structure

```
src/
├── components/     # Reusable UI components
├── pages/         # Page components
├── hooks/         # Custom React hooks
├── services/      # API and WebSocket services
├── types/         # TypeScript type definitions
├── utils/         # Utility functions
├── styles/        # Global styles
├── test/          # Test setup and utilities
└── assets/        # Static assets
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_BASE_URL` | API base URL | `http://localhost:8080/api/v1` |
| `VITE_WS_URL` | WebSocket URL | `ws://localhost:8080/api/v1` |
| `VITE_AUTH_TOKEN_KEY` | LocalStorage key for auth token | `laforge_auth_token` |
| `VITE_ENV` | Environment | `development` |

## API Integration

The web UI integrates with the LaForge API server (`laserve`) to provide:

- Task management (CRUD operations)
- Task status updates
- Review workflow
- Step history
- Real-time updates via WebSocket

## Development Guidelines

### Code Style

- Use TypeScript for all new code
- Follow the existing component structure
- Use functional components with hooks
- Keep components small and focused
- Use proper TypeScript types

### Testing

- Write unit tests for utilities and hooks
- Test components with @testing-library/preact
- Aim for >80% test coverage
- Run tests before committing

### Git Workflow

1. Create feature branches from `main`
2. Write descriptive commit messages
3. Run linting and tests before pushing
4. Create pull requests for review

## Production Deployment

1. Build the application:
   ```bash
   npm run build
   ```

2. The built files will be in the `dist/` directory

3. Serve the `dist/` directory with your web server

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run linting and tests
6. Submit a pull request

## License

Same as LaForge project.