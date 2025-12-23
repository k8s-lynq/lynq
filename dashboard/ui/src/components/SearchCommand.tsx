import { useEffect, useState, useCallback } from "react"
import { useNavigate } from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command"
import {
  IconDatabase,
  IconFileCode,
  IconBox,
  IconCircleCheck,
  IconCircleX,
  IconClock,
  IconCircleMinus,
  IconLayoutDashboard,
  IconSitemap,
} from "@tabler/icons-react"
import { useHubs } from "@/contexts/HubContext"
import { useForms } from "@/contexts/FormContext"
import { useNodes } from "@/contexts/NodeContext"
import type { ResourceStatus } from "@/types/lynq"

const STATUS_ICONS: Record<ResourceStatus, React.ReactNode> = {
  ready: <IconCircleCheck size={12} className="text-emerald-500" />,
  failed: <IconCircleX size={12} className="text-rose-500" />,
  pending: <IconClock size={12} className="text-amber-500" />,
  skipped: <IconCircleMinus size={12} className="text-slate-400" />,
}

function getStatus(conditions?: { type: string; status: string }[]): ResourceStatus {
  if (!conditions) return "pending"
  const readyCondition = conditions.find(
    (c) => c.type === "Ready" || c.type === "Applied"
  )
  if (!readyCondition) return "pending"
  if (readyCondition.status === "True") return "ready"
  if (readyCondition.status === "False") return "failed"
  return "pending"
}

export function SearchCommand() {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const { t } = useTranslation()

  const { hubs } = useHubs()
  const { forms } = useForms()
  const { nodes } = useNodes()

  // Global keyboard shortcut
  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setOpen((open) => !open)
      }
    }

    document.addEventListener("keydown", down)
    return () => document.removeEventListener("keydown", down)
  }, [])

  const runCommand = useCallback((command: () => void) => {
    setOpen(false)
    command()
  }, [])

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder={t('search.searchHubsFormsNodes')} />
      <CommandList>
        <CommandEmpty>{t('search.noResults')}</CommandEmpty>

        {/* Quick Navigation */}
        <CommandGroup heading={t('search.navigation')}>
          <CommandItem
            onSelect={() => runCommand(() => navigate("/"))}
          >
            <IconLayoutDashboard size={16} className="mr-2" />
            {t('nav.overview')}
          </CommandItem>
          <CommandItem
            onSelect={() => runCommand(() => navigate("/topology"))}
          >
            <IconSitemap size={16} className="mr-2" />
            {t('nav.topology')}
          </CommandItem>
          <CommandItem
            onSelect={() => runCommand(() => navigate("/hubs"))}
          >
            <IconDatabase size={16} className="mr-2" />
            {t('search.allHubs')}
          </CommandItem>
          <CommandItem
            onSelect={() => runCommand(() => navigate("/forms"))}
          >
            <IconFileCode size={16} className="mr-2" />
            {t('search.allForms')}
          </CommandItem>
          <CommandItem
            onSelect={() => runCommand(() => navigate("/nodes"))}
          >
            <IconBox size={16} className="mr-2" />
            {t('search.allNodes')}
          </CommandItem>
        </CommandGroup>

        <CommandSeparator />

        {/* Hubs */}
        {hubs.length > 0 && (
          <CommandGroup heading={t('nav.hubs')}>
            {hubs.map((hub) => {
              const status = getStatus(hub.status?.conditions)
              return (
                <CommandItem
                  key={`hub-${hub.metadata.namespace}-${hub.metadata.name}`}
                  onSelect={() =>
                    runCommand(() =>
                      navigate(`/hubs/${hub.metadata.name}?namespace=${hub.metadata.namespace}`)
                    )
                  }
                >
                  <IconDatabase size={16} className="mr-2" />
                  <span className="flex-1">{hub.metadata.name}</span>
                  <span className="text-xs text-muted-foreground mr-2">
                    {hub.metadata.namespace}
                  </span>
                  {STATUS_ICONS[status]}
                </CommandItem>
              )
            })}
          </CommandGroup>
        )}

        {/* Forms */}
        {forms.length > 0 && (
          <CommandGroup heading={t('nav.forms')}>
            {forms.map((form) => {
              const status = getStatus(form.status?.conditions)
              return (
                <CommandItem
                  key={`form-${form.metadata.namespace}-${form.metadata.name}`}
                  onSelect={() =>
                    runCommand(() =>
                      navigate(`/forms/${form.metadata.name}?namespace=${form.metadata.namespace}`)
                    )
                  }
                >
                  <IconFileCode size={16} className="mr-2" />
                  <span className="flex-1">{form.metadata.name}</span>
                  <span className="text-xs text-muted-foreground mr-2">
                    {form.metadata.namespace}
                  </span>
                  {STATUS_ICONS[status]}
                </CommandItem>
              )
            })}
          </CommandGroup>
        )}

        {/* Nodes */}
        {nodes.length > 0 && (
          <CommandGroup heading={t('nav.nodes')}>
            {nodes.slice(0, 20).map((node) => {
              const status = getStatus(node.status?.conditions)
              return (
                <CommandItem
                  key={`node-${node.metadata.namespace}-${node.metadata.name}`}
                  onSelect={() =>
                    runCommand(() =>
                      navigate(`/nodes/${node.metadata.name}?namespace=${node.metadata.namespace}`)
                    )
                  }
                >
                  <IconBox size={16} className="mr-2" />
                  <span className="flex-1 truncate">{node.metadata.name}</span>
                  <span className="text-xs text-muted-foreground mr-2">
                    {node.metadata.namespace}
                  </span>
                  {STATUS_ICONS[status]}
                </CommandItem>
              )
            })}
            {nodes.length > 20 && (
              <CommandItem
                onSelect={() => runCommand(() => navigate("/nodes"))}
              >
                <span className="text-muted-foreground">
                  {t('search.moreNodes', { count: nodes.length - 20 })}
                </span>
              </CommandItem>
            )}
          </CommandGroup>
        )}
      </CommandList>
    </CommandDialog>
  )
}
