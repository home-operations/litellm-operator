package controller

import (
	"context"
	"fmt"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
	"github.com/home-operations/litellm-operator/internal/litellmclient"
)

const teamFinalizer = "litellm.home-operations.com/team"

type LiteLLMTeamReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmteams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmteams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmteams/finalizers,verbs=update
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmproxies,verbs=get;list;watch

func (r *LiteLLMTeamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var team litellmv1alpha1.LiteLLMTeam
	if err := r.Get(ctx, req.NamespacedName, &team); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !team.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.delete(ctx, &team)
	}
	if !controllerutil.ContainsFinalizer(&team, teamFinalizer) {
		controllerutil.AddFinalizer(&team, teamFinalizer)
		if err := r.Update(ctx, &team); err != nil {
			return ctrl.Result{}, err
		}
	}
	admin, err := r.adminClient(ctx, &team)
	if err != nil {
		return ctrl.Result{}, r.failed(ctx, &team, "AdminClientFailed", err.Error())
	}
	body, err := teamRequest(&team)
	if err != nil {
		return ctrl.Result{}, r.failed(ctx, &team, "InvalidSpec", err.Error())
	}
	if team.Status.TeamID == "" {
		created, err := admin.CreateTeam(ctx, body)
		if err != nil {
			return ctrl.Result{}, r.failed(ctx, &team, "CreateFailed", err.Error())
		}
		team.Status.TeamID = created.TeamID
	} else if ready := meta.FindStatusCondition(team.Status.Conditions, conditionTypeReady); ready == nil || ready.ObservedGeneration != team.Generation {
		body.TeamID = team.Status.TeamID
		if err := admin.UpdateTeam(ctx, body); err != nil {
			return ctrl.Result{}, r.failed(ctx, &team, "UpdateFailed", err.Error())
		}
		if err := r.members(ctx, admin, team.Status.TeamID, team.Spec.Members); err != nil {
			return ctrl.Result{}, r.failed(ctx, &team, "MembersFailed", err.Error())
		}
	}
	return ctrl.Result{}, r.ready(ctx, &team)
}

func (r *LiteLLMTeamReconciler) delete(ctx context.Context, team *litellmv1alpha1.LiteLLMTeam) error {
	if !controllerutil.ContainsFinalizer(team, teamFinalizer) {
		return nil
	}
	if team.Status.TeamID == "" {
		controllerutil.RemoveFinalizer(team, teamFinalizer)
		return r.Update(ctx, team)
	}
	admin, err := r.adminClient(ctx, team)
	if err != nil {
		return err
	}
	if err := admin.DeleteTeam(ctx, team.Status.TeamID); err != nil {
		return fmt.Errorf("delete LiteLLM team: %w", err)
	}
	controllerutil.RemoveFinalizer(team, teamFinalizer)
	return r.Update(ctx, team)
}
func (r *LiteLLMTeamReconciler) members(ctx context.Context, admin *litellmclient.Client, id string, wanted []litellmv1alpha1.LiteLLMTeamMember) error {
	remote, err := admin.GetTeam(ctx, id)
	if err != nil {
		return err
	}
	existing := map[string]litellmclient.TeamMember{}
	for _, m := range remote.Members {
		existing[memberID(m.UserID, m.UserEmail)] = m
	}
	desired := map[string]litellmv1alpha1.LiteLLMTeamMember{}
	for _, m := range wanted {
		desired[memberID(m.UserID, m.UserEmail)] = m
	}
	for id, m := range existing {
		d, ok := desired[id]
		if !ok || d.Role != m.Role {
			if err := admin.DeleteTeamMember(ctx, remote.TeamID, m); err != nil {
				return err
			}
		}
	}
	for id, m := range desired {
		old, ok := existing[id]
		if !ok || old.Role != m.Role {
			if err := admin.AddTeamMember(ctx, remote.TeamID, litellmclient.TeamMember{UserID: m.UserID, UserEmail: m.UserEmail, Role: m.Role}); err != nil {
				return err
			}
		}
	}
	return nil
}

func memberID(id, email string) string {
	if id != "" {
		return id
	}
	return email
}

func (r *LiteLLMTeamReconciler) adminClient(ctx context.Context, team *litellmv1alpha1.LiteLLMTeam) (*litellmclient.Client, error) {
	var proxy litellmv1alpha1.LiteLLMProxy
	if err := r.Get(ctx, types.NamespacedName{Namespace: team.Namespace, Name: team.Spec.ProxyRef}, &proxy); err != nil {
		return nil, err
	}
	if proxy.Spec.APIAccess == nil {
		return nil, fmt.Errorf("proxy %s has no spec.apiAccess", proxy.Name)
	}
	key, err := readSecretKey(ctx, r.Client, team.Namespace, proxy.Spec.APIAccess.MasterKeyRef)
	if err != nil {
		return nil, err
	}
	endpoint := proxy.Spec.APIAccess.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("http://%s.%s.svc:%d", proxy.Name, proxy.Namespace, servicePort(&proxy))
	}
	return litellmclient.New(endpoint, key, nil), nil
}

func teamRequest(team *litellmv1alpha1.LiteLLMTeam) (litellmclient.TeamRequest, error) {
	out := litellmclient.TeamRequest{TeamID: team.DesiredTeamID(), TeamAlias: team.Spec.Alias, Models: team.Spec.Models, BudgetDuration: team.Spec.BudgetDuration, TPMLimit: team.Spec.TPMLimit, RPMLimit: team.Spec.RPMLimit, Metadata: team.Spec.Metadata, Blocked: team.Spec.Blocked}
	out.ObjectPermission = &litellmclient.TeamObjectPermission{MCPServers: team.Spec.MCPServers}
	for _, m := range team.Spec.Members {
		if memberID(m.UserID, m.UserEmail) == "" {
			return out, fmt.Errorf("member requires userID or userEmail")
		}
		out.Members = append(out.Members, litellmclient.TeamMember{UserID: m.UserID, UserEmail: m.UserEmail, Role: m.Role})
	}
	if team.Spec.MaxBudget != "" {
		v, err := strconv.ParseFloat(team.Spec.MaxBudget, 64)
		if err != nil {
			return out, err
		}
		out.MaxBudget = &v
	}
	return out, nil
}

func (r *LiteLLMTeamReconciler) ready(ctx context.Context, team *litellmv1alpha1.LiteLLMTeam) error {
	meta.SetStatusCondition(&team.Status.Conditions, metav1.Condition{Type: conditionTypeReady, Status: metav1.ConditionTrue, Reason: conditionReasonReconciled, Message: "team reconciled", ObservedGeneration: team.Generation})
	return r.Status().Update(ctx, team)
}

func (r *LiteLLMTeamReconciler) failed(ctx context.Context, team *litellmv1alpha1.LiteLLMTeam, reason, message string) error {
	meta.SetStatusCondition(&team.Status.Conditions, metav1.Condition{Type: conditionTypeReady, Status: metav1.ConditionFalse, Reason: reason, Message: message, ObservedGeneration: team.Generation})
	if err := r.Status().Update(ctx, team); err != nil && !apierrors.IsConflict(err) {
		return err
	}
	return fmt.Errorf("%s: %s", reason, message)
}

func (r *LiteLLMTeamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&litellmv1alpha1.LiteLLMTeam{}).Complete(r)
}
