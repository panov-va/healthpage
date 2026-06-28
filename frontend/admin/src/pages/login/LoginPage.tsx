import { useNavigate } from "react-router-dom";

import { AuthForm } from "@/features/auth";
import { Card } from "@/shared/ui";

export function LoginPage() {
  const navigate = useNavigate();
  return (
    <div className="hp-center">
      <Card>
        <AuthForm onSuccess={() => navigate("/", { replace: true })} />
      </Card>
    </div>
  );
}
