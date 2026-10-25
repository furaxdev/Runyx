// Single place every icon used in the UI is imported from (lucide-svelte
// — ISC licensed, see THIRD_PARTY.md). One icon set, one stroke style,
// one weight everywhere: never an emoji character, never a raw glyph.
export {
  MessageSquare,
  Package,
  Smartphone,
  Settings as SettingsIcon,
  ShieldCheck,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  FolderClosed,
  ChevronDown,
  ChevronRight,
  Search,
  Cpu,
  Download,
  Pause,
  Play,
  Trash2,
  Check,
  X,
  Star,
  Mic,
  Square,
  SendHorizontal,
  Volume2,
  PenLine,
  BookOpen,
  Code,
  FileText,
  Eye,
  Wrench,
  Brain,
  Globe,
  AlignLeft,
  QrCode,
  Copy,
  RefreshCw,
  Pencil,
  Clock,
  ArrowDown,
  Keyboard,
} from "lucide-svelte";

import type { ComponentType } from "svelte";
import { Hexagon, Wind, Gem, Compass, Pi, Infinity as InfinitySymbol, Cpu as CpuGlyph } from "lucide-svelte";

/**
 * Maps a model's display name/family/repo string to a fitting abstract
 * icon -- one glyph per well-known model family, purely by loose
 * metaphor, never a redrawn vendor logo (Llama/Qwen/Mistral/Gemma/etc.
 * are trademarked marks a third-party open-source project shouldn't
 * reproduce): Qwen -> hexagon, Mistral/Mixtral -> wind ("mistral" is
 * French for a strong wind), Gemma/Gemini -> gem, DeepSeek -> compass
 * ("seek"), Phi -> the Greek letter pi (the closest available glyph),
 * Llama/Meta -> an infinity loop (nods to Meta's own branding motif
 * without reproducing it), everything else -> a generic chip icon.
 */
export function modelFamilyIcon(nameOrFamily: string): ComponentType {
  const s = nameOrFamily.toLowerCase();
  if (s.includes("qwen")) return Hexagon;
  if (s.includes("mistral") || s.includes("mixtral")) return Wind;
  if (s.includes("gemma") || s.includes("gemini")) return Gem;
  if (s.includes("deepseek")) return Compass;
  if (s.includes("phi")) return Pi;
  if (s.includes("llama") || s.includes("meta")) return InfinitySymbol;
  return CpuGlyph;
}
