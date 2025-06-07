import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { LoginForm } from './LoginForm'

describe('LoginForm', () => {
  it('calls onLogin when credentials are valid', async () => {
    const onLogin = jest.fn()
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => ({}) }) as any

    render(<LoginForm onLogin={onLogin} />)
    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'p' } })
    fireEvent.click(screen.getByRole('button', { name: /login/i }))

    await waitFor(() => expect(onLogin).toHaveBeenCalled())
  })
})
