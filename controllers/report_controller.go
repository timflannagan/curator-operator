/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	curatorv1alpha1 "github.com/operate-first/curator-operator/api/v1alpha1"
	"github.com/operate-first/curator-operator/internal/reporting"
)

// ReportReconciler reconciles a Report object
type ReportReconciler struct {
	client.Client
	DB     *pgx.Conn
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=reports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=reports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=reports/finalizers,verbs=update
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=curatorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=curatorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=curatorconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("reconciling report", "req", req.NamespacedName)
	defer l.Info("finished reconciling report", "req", req.NamespacedName)

	report := &curatorv1alpha1.Report{}
	if err := r.Client.Get(ctx, req.NamespacedName, report); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	now := time.Now().UTC()
	reportPeriod, err := reporting.GetReportPeriod(now, l, report)
	if err != nil {
		return ctrl.Result{}, nil
	}
	if reportPeriod.PeriodEnd.After(now) { // @fixme
		return ctrl.Result{RequeueAfter: reportPeriod.PeriodEnd.Sub(now)}, nil
	}

	report.Status.LastReportTime = &metav1.Time{Time: reportPeriod.PeriodEnd}
	if err := r.Status().Update(ctx, report); err != nil {
		l.Info("reconciling report", "Update Err", err)
		return ctrl.Result{}, err
	}
	if report.Spec.Schedule == nil {
		return ctrl.Result{}, nil
	}
	reportSchedule, err := reporting.GetSchedule(report.Spec.Schedule)
	if err != nil {
		return ctrl.Result{}, err // @fixme empty results ?
	}
	nextReportPeriod := reporting.GetNextReportPeriod(reportSchedule, report.Status.LastReportTime.Time)

	// update the NextReportTime on the report status
	report.Status.NextReportTime = &metav1.Time{Time: nextReportPeriod.PeriodEnd}
	now = time.Now().UTC()
	nextRunTime := nextReportPeriod.PeriodEnd
	waitTime := nextRunTime.Sub(now)

	/*
		TODO:
		- [x] Create a Report status.tableRef (string?)
		- [ ] If the report.Status.TableRef != nil: verify the table exists, else created it and update the status
		- [ ] Else: create the table and update the report.Status.TableRef reference to that created table
		- [ ] Add support for emulating the select query used in operate-first/curator/apis/scripts/app.py and handle errors

		Steps:
		- Build up a SQL query that selects data from the logs_0 table based on the Report inputs (e.g. periodStart, periodEnd, namespace, etc.)
		- Fire off that SQL query using the r.DB client and scan row results
	*/

	var (
		reportPeriodStart time.Time
		reportPeriodEnd   time.Time
		namespace         string
		usage             float64
	)
	query := "SELECT report_period_start, report_period_end, namespace, pod_usage_cpu_core_seconds FROM logs_2 WHERE namespace='openshift-metering' limit 5;"
	err = r.DB.QueryRow(ctx, query).Scan(&reportPeriodStart, &reportPeriodEnd, &namespace, &usage)
	if err != nil {
		l.Error(err, "failed to exec query", "query", query)
		return ctrl.Result{}, err
	}
	l.Info("query results", "start", reportPeriodStart, "end", reportPeriodEnd, "namespace", namespace, "usage", usage)

	if err := r.Status().Update(ctx, report); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: waitTime}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&curatorv1alpha1.Report{}).
		Watches(&source.Kind{Type: &curatorv1alpha1.CuratorConfig{}}, requeueReportsHandler(mgr.GetClient(), mgr.GetLogger())).
		Complete(r)
}

func requeueReportsHandler(c client.Client, log logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		reports := &curatorv1alpha1.ReportList{}
		if err := c.List(context.Background(), reports); err != nil {
			return nil
		}

		var res []reconcile.Request
		for _, report := range reports.Items {
			log.Info("requeuing report", "name", report.GetName(), "ns", report.GetNamespace())
			res = append(res, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&report),
			})
		}
		return res
	})
}

// @app.route('/report')
// def report():
//     resp = get_report(request.args)  # TODO status code
//     # print(resp, file=sys.stdout)
//     if 'reportingStart' in resp['spec'].keys():
//         sql = "SELECT * FROM logs_2 WHERE interval_start >= '{}'::timestamp with time zone AND interval_end < '{}'::timestamp with time zone "\
//             .format(resp['spec']['reportingStart'],resp['spec']['reportingEnd'])
//     else:
//         offset = 0
//         if resp['spec']['reportPeriod'].lower() == 'day':
//             offset = 1
//         elif resp['spec']['reportPeriod'].lower() == 'week':
//             offset = 7
//         elif resp['spec']['reportPeriod'].lower() == 'month':
//             offset = 30
//         sql = "SELECT * FROM logs_2 WHERE interval_start >= '{0}'::timestamp with time zone - interval '{1} day' AND interval_end < '{0}'::timestamp with time zone".\
//             format(resp['spec']['reportingEnd'], offset)
//     if 'namespace' in resp['spec'].keys():
//         sql += " AND namespace='{}'".format(resp['spec']['namespace'])
//     # print(sql, file=sys.stdout)
//     table = postgres_execute(sql, result=True, header=True)
//     return jsonify(table)
//     # df = pd.DataFrame(table[1:])
//     # df.columns = table[0]
//     # html_template = Template(TEMPLATE_CONTENT)
//     # return html_template.render(**{"tables": [df.to_html(classes='data')], "titles": df.columns.values})
