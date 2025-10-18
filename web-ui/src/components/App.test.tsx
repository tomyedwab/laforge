import { render, screen } from '@testing-library/preact';
import { App } from '../app';

describe('App', () => {
  it('renders headline', () => {
    render(<App />);
    expect(screen.getByText('Vite + Preact')).toBeInTheDocument();
  });

  it('renders count button', () => {
    render(<App />);
    const button = screen.getByRole('button', { name: /count is 0/i });
    expect(button).toBeInTheDocument();
  });

  it('increments count when button is clicked', () => {
    render(<App />);
    const button = screen.getByRole('button', { name: /count is 0/i });
    button.click();
    expect(screen.getByRole('button', { name: /count is 1/i })).toBeInTheDocument();
  });
});