import { logout, useSession } from "@/entities/session";
import { Button } from "@/shared/ui";

export function LogoutButton() {
  const { clear } = useSession();

  async function handleLogout() {
    try {
      await logout();
    } catch {
      /* отзыв refresh на сервере не критичен для UX — всё равно чистим локально */
    }
    clear();
  }

  return (
    <Button variant="secondary" size="sm" onClick={handleLogout}>
      Выйти
    </Button>
  );
}
