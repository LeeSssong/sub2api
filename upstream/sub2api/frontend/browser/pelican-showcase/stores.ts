export const useAuthStore = () => ({ isAdmin: true })
export const useAppStore = () => ({ showError: (message: string) => console.error(message), showSuccess: () => {} })
